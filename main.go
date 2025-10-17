package main

import (
	"fmt"
	"log"
	"os"
	"thaimaster2d/admin"
	"thaimaster2d/appconfig"
	"thaimaster2d/gift"
	"thaimaster2d/live"
	"thaimaster2d/paper"
	"thaimaster2d/slider"
	"thaimaster2d/threed"
	"thaimaster2d/twodhistory"

	"github.com/gin-gonic/gin"
)

func main() {
	// Create Gin router
	r := gin.Default()

	// Enable CORS for all origins
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Initialize database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Default local PostgreSQL connection
		dbURL = "postgres://postgres:postgres@localhost:5432/thaimaster2d?sslmode=disable"
	}

	dbEnabled := false
	if err := twodhistory.InitDB(dbURL); err != nil {
		log.Printf("⚠️  Database initialization failed: %v", err)
		log.Println("⚠️  Continuing without database features...")
	} else {
		defer twodhistory.CloseDB()
		dbEnabled = true

		// Initialize gift and slider packages
		db := twodhistory.GetDB()
		gift.InitDB(db)
		slider.InitDB(db)
		admin.InitDB(db)
		threed.InitDB(db)
		appconfig.InitDB(db)
		paper.InitDB(db)
	}

	// Initialize live package
	live.Init()

	// Register history inserter callback if database is enabled
	if dbEnabled {
		live.SetHistoryInserter(func(data *live.LotteryData) error {
			// Convert live.LotteryData to twodhistory.LotteryData
			histData := &twodhistory.LotteryData{
				Date:        data.Date,
				Live:        data.Live,
				Status:      data.Status,
				Set1200:     data.Set1200,
				Value1200:   data.Value1200,
				Result1200:  data.Result1200,
				Set430:      data.Set430,
				Value430:    data.Value430,
				Result430:   data.Result430,
				Modern930:   data.Modern930,
				Internet930: data.Internet930,
				Modern200:   data.Modern200,
				Internet200: data.Internet200,
				UpdateTime:  data.UpdateTime,
			}
			return twodhistory.InsertFromLotteryData(histData)
		})
		log.Println("✅ History auto-insert enabled (16:30-16:35 GMT+6:30)")
	}

	// Routes
	r.POST("/api/lottery/update", live.UpdateLotteryData)
	r.GET("/api/lottery/stream", live.StreamLotteryData)
	r.GET("/api/lottery/current", live.GetCurrentData)

	// History routes
	r.GET("/api/twodhistory", twodhistory.GetHistoryHandler)
	r.POST("/api/twodhistory/check", twodhistory.CheckAndInsertHandler)

	// Gift routes
	r.GET("/api/gifts", gift.GetGiftsHandler)

	// Slider routes
	r.GET("/api/sliders", slider.GetSlidersHandler)

	// 3D routes
	r.GET("/api/threed", threed.GetAllResults)
	r.POST("/api/threed", threed.CreateResult)
	r.PUT("/api/threed", threed.UpdateResult)
	r.DELETE("/api/threed", threed.DeleteResult)

	// Paper routes (public)
	r.GET("/api/paper/types", paper.GetAllTypes)
	r.GET("/api/paper/types/:type_id/images", paper.GetImagesByType)

	// App Config routes (public)
	r.GET("/api/appconfig", appconfig.GetAppConfig)
	r.GET("/api/appconfig/check", appconfig.CheckVersion)

	// Admin routes
	if dbEnabled {
		// Serve uploaded files
		r.Static("/uploads", "./uploads")

		// Load HTML templates
		r.LoadHTMLGlob("admin/templates/*.html")

		// Admin dashboard pages
		r.GET("/admin", admin.AdminDashboardHandler)
		r.GET("/admin/gifts", admin.ManageGiftsPageHandler)
		r.GET("/admin/sliders", admin.ManageSlidersPageHandler)
		r.GET("/admin/threed", admin.ManageThreeDPageHandler)
		r.GET("/admin/paper", admin.ManagePaperPageHandler)
		r.GET("/admin/appconfig", admin.AppConfigPageHandler)
		r.POST("/admin/appconfig/update", admin.UpdateAppConfigHandler)
		r.GET("/admin/gifts/create", admin.CreateGiftPageHandler)
		r.GET("/admin/sliders/create", admin.CreateSliderPageHandler)
		r.GET("/admin/threed/create", admin.CreateThreeDPageHandler)
		r.POST("/admin/threed/create", admin.CreateThreeDHandler)
		r.GET("/admin/gifts/edit/:id", admin.EditGiftPageHandler)
		r.GET("/admin/sliders/edit/:id", admin.EditSliderPageHandler)
		r.GET("/admin/threed/edit", admin.EditThreeDPageHandler)
		r.POST("/admin/threed/edit", admin.EditThreeDHandler)
		r.POST("/admin/threed/delete", admin.DeleteThreeDHandler)

		// Image upload routes
		r.POST("/api/admin/upload-image", admin.UploadImageHandler)
		r.DELETE("/api/admin/delete-image/:filename", admin.DeleteImageHandler) // Admin API routes for gifts
		r.GET("/api/admin/gifts", func(c *gin.Context) {
			gifts, err := gift.GetAllGiftsForAdmin()
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gifts)
		})
		r.GET("/api/admin/gifts/:id", admin.GetGiftByIDHandler)
		r.POST("/api/admin/gifts", func(c *gin.Context) {
			var newGift gift.Gift
			if err := c.BindJSON(&newGift); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			if err := gift.InsertGift(newGift); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"message": "Gift created"})
		})
		r.PUT("/api/admin/gifts/:id", func(c *gin.Context) {
			var updatedGift gift.Gift
			if err := c.BindJSON(&updatedGift); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			if err := gift.UpdateGift(updatedGift); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"message": "Gift updated"})
		})
		r.DELETE("/api/admin/gifts/:id", func(c *gin.Context) {
			var id int
			if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil {
				c.JSON(400, gin.H{"error": "Invalid ID"})
				return
			}
			if err := gift.DeleteGift(id); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"message": "Gift deleted"})
		})

		// Admin API routes for sliders
		r.GET("/api/admin/sliders", func(c *gin.Context) {
			sliders, err := slider.GetAllSlidersForAdmin()
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, sliders)
		})
		r.GET("/api/admin/sliders/:id", admin.GetSliderByIDHandler)
		r.POST("/api/admin/sliders", func(c *gin.Context) {
			var newSlider slider.Slider
			if err := c.BindJSON(&newSlider); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			if err := slider.InsertSlider(newSlider); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"message": "Slider created"})
		})
		r.PUT("/api/admin/sliders/:id", func(c *gin.Context) {
			var updatedSlider slider.Slider
			if err := c.BindJSON(&updatedSlider); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			if err := slider.UpdateSlider(updatedSlider); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"message": "Slider updated"})
		})
		r.DELETE("/api/admin/sliders/:id", func(c *gin.Context) {
			var id int
			if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil {
				c.JSON(400, gin.H{"error": "Invalid ID"})
				return
			}
			if err := slider.DeleteSlider(id); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"message": "Slider deleted"})
		})

		// Admin API routes for paper
		r.GET("/api/admin/paper/types", paper.GetAllTypesWithImages)
		r.POST("/api/admin/paper/types", paper.CreateType)
		r.PUT("/api/admin/paper/types/:id", paper.UpdateType)
		r.DELETE("/api/admin/paper/types/:id", paper.DeleteType)
		r.POST("/api/admin/paper/images", paper.CreateImage)
		r.POST("/api/admin/paper/images/batch", paper.BatchCreateImages)
		r.PUT("/api/admin/paper/images/:id", paper.UpdateImage)
		r.DELETE("/api/admin/paper/images/:id", paper.DeleteImage)
	}

	// Health check
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "ThaiMaster2D Lottery API Server",
			"version": "1.0.0",
		})
	})

	// Start server
	log.Println("🚀 Server starting on :4545")
	log.Println("📡 SSE Stream available at: http://localhost:4545/api/lottery/stream")
	log.Println("📮 POST lottery data to: http://localhost:4545/api/lottery/update")
	log.Println("📜 History data at: http://localhost:4545/api/twodhistory")
	if err := r.Run(":4545"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
