package admin

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var db *sql.DB

// InitDB initializes the database connection for admin
func InitDB(database *sql.DB) {
	db = database
}

// AdminDashboardHandler renders the admin dashboard home
func AdminDashboardHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title": "Admin Dashboard - ThaiMaster2D",
	})
}

// ManageGiftsPageHandler renders the gifts management page
func ManageGiftsPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "manage_gifts.html", gin.H{
		"title": "Manage Gifts - Admin",
	})
}

// ManageSlidersPageHandler renders the sliders management page
func ManageSlidersPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "manage_sliders.html", gin.H{
		"title": "Manage Sliders - Admin",
	})
}

// CreateGiftPageHandler renders the create gift form
func CreateGiftPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "create_gift.html", gin.H{
		"title": "Create Gift - Admin",
	})
}

// CreateSliderPageHandler renders the create slider form
func CreateSliderPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "create_slider.html", gin.H{
		"title": "Create Slider - Admin",
	})
}

// EditGiftPageHandler renders the edit gift form
func EditGiftPageHandler(c *gin.Context) {
	id := c.Param("id")
	c.HTML(http.StatusOK, "edit_gift.html", gin.H{
		"title": "Edit Gift - Admin",
		"id":    id,
	})
}

// EditSliderPageHandler renders the edit slider form
func EditSliderPageHandler(c *gin.Context) {
	id := c.Param("id")
	c.HTML(http.StatusOK, "edit_slider.html", gin.H{
		"title": "Edit Slider - Admin",
		"id":    id,
	})
}

// GetGiftByIDHandler returns a single gift by ID
func GetGiftByIDHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	query := `
		SELECT id, name, image_link, type, description, points, stock, is_active, created_at
		FROM gifts WHERE id = $1
	`
	var gift struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		ImageLink   string `json:"image_link"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Points      int    `json:"points"`
		Stock       int    `json:"stock"`
		IsActive    bool   `json:"is_active"`
		CreatedAt   string `json:"created_at"`
	}

	err = db.QueryRow(query, id).Scan(&gift.ID, &gift.Name, &gift.ImageLink,
		&gift.Type, &gift.Description, &gift.Points, &gift.Stock, &gift.IsActive, &gift.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gift not found"})
		return
	}

	c.JSON(http.StatusOK, gift)
}

// GetSliderByIDHandler returns a single slider by ID
func GetSliderByIDHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	query := `
		SELECT id, image_link, forward_link, title, order_num, is_active, created_at
		FROM sliders WHERE id = $1
	`
	var slider struct {
		ID          int    `json:"id"`
		ImageLink   string `json:"image_link"`
		ForwardLink string `json:"forward_link"`
		Title       string `json:"title"`
		Order       int    `json:"order"`
		IsActive    bool   `json:"is_active"`
		CreatedAt   string `json:"created_at"`
	}

	err = db.QueryRow(query, id).Scan(&slider.ID, &slider.ImageLink, &slider.ForwardLink,
		&slider.Title, &slider.Order, &slider.IsActive, &slider.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Slider not found"})
		return
	}

	c.JSON(http.StatusOK, slider)
}

// UploadImageHandler handles image uploads and returns the file path
func UploadImageHandler(c *gin.Context) {
	// Get the file from form data
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image file provided"})
		return
	}

	// Validate file type
	ext := filepath.Ext(file.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only jpg, png, gif, webp allowed"})
		return
	}

	// Create uploads directory if not exists
	uploadsDir := "uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create uploads directory"})
		return
	}

	// Generate unique filename using timestamp
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, filepath.Base(file.Filename))
	filepath := filepath.Join(uploadsDir, filename)

	// Save the file
	if err := c.SaveUploadedFile(file, filepath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
		return
	}

	// Get the host from the request to build full URL
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	host := c.Request.Host
	
	// Return the full image URL via API endpoint (not static /uploads)
	imageURL := fmt.Sprintf("%s://%s/api/images/%s", scheme, host, filename)
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"image_url": imageURL,
		"filename":  filename,
	})
}

// DeleteImageHandler deletes an uploaded image file
func DeleteImageHandler(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename required"})
		return
	}

	// Construct file path
	filepath := filepath.Join("uploads", filename)

	// Check if file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Delete the file
	if err := os.Remove(filepath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Image deleted"})
}

// ManageThreeDPageHandler renders the 3D results management page
func ManageThreeDPageHandler(c *gin.Context) {
	rows, err := db.Query(`
		SELECT id, date, result, created_at, updated_at 
		FROM threed 
		ORDER BY date DESC
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "manage_threed.html", gin.H{
			"Error": "Failed to fetch 3D results",
		})
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var date, result string
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &date, &result, &createdAt, &updatedAt); err != nil {
			continue
		}

		results = append(results, map[string]interface{}{
			"ID":        id,
			"Date":      date,
			"Result":    result,
			"CreatedAt": createdAt,
			"UpdatedAt": updatedAt,
		})
	}

	c.HTML(http.StatusOK, "manage_threed.html", gin.H{
		"title":   "Manage 3D Results - Admin",
		"Results": results,
	})
}

// ManagePaperPageHandler renders the paper management page
func ManagePaperPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "manage_paper.html", gin.H{
		"title": "Manage Paper - Admin",
	})
}

// CreateThreeDPageHandler renders the create 3D result form
func CreateThreeDPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "create_threed.html", gin.H{
		"title": "Create 3D Result - Admin",
		"Today": time.Now().Format("2006-01-02"),
	})
}

// CreateThreeDHandler handles creating a new 3D result
func CreateThreeDHandler(c *gin.Context) {
	date := c.PostForm("date")
	result := c.PostForm("result")

	// Validate inputs
	if date == "" || result == "" {
		c.HTML(http.StatusBadRequest, "create_threed.html", gin.H{
			"Error": "All fields are required",
			"Today": time.Now().Format("2006-01-02"),
		})
		return
	}

	if len(result) != 3 {
		c.HTML(http.StatusBadRequest, "create_threed.html", gin.H{
			"Error": "Result must be exactly 3 digits",
			"Today": time.Now().Format("2006-01-02"),
		})
		return
	}

	// Insert into database
	query := `
		INSERT INTO threed (date, result, created_at, updated_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	_, err := db.Exec(query, date, result)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "create_threed.html", gin.H{
			"Error": "Failed to create result. Date might already exist.",
			"Today": time.Now().Format("2006-01-02"),
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/threed?message=Result created successfully")
}

// EditThreeDPageHandler renders the edit 3D result form
func EditThreeDPageHandler(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/threed")
		return
	}

	var result struct {
		ID        int
		Date      string
		Result    string
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	var date time.Time

	err = db.QueryRow("SELECT id, date, result, created_at, updated_at FROM threed WHERE id = $1", id).
		Scan(&result.ID, &date, &result.Result, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/threed")
		return
	}

	result.Date = date.Format("2006-01-02")

	c.HTML(http.StatusOK, "edit_threed.html", gin.H{
		"title":  "Edit 3D Result - Admin",
		"Result": result,
	})
}

// EditThreeDHandler handles updating a 3D result
func EditThreeDHandler(c *gin.Context) {
	idStr := c.PostForm("id")
	result := c.PostForm("result")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/threed")
		return
	}

	if len(result) != 3 {
		c.HTML(http.StatusBadRequest, "edit_threed.html", gin.H{
			"Error": "Result must be exactly 3 digits",
		})
		return
	}

	query := `UPDATE threed SET result = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err = db.Exec(query, result, id)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "edit_threed.html", gin.H{
			"Error": "Failed to update result",
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/threed?message=Result updated successfully")
}

// DeleteThreeDHandler handles deleting a 3D result
func DeleteThreeDHandler(c *gin.Context) {
	idStr := c.PostForm("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/threed")
		return
	}

	_, err = db.Exec("DELETE FROM threed WHERE id = $1", id)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/threed?message=Failed to delete result")
		return
	}

	c.Redirect(http.StatusFound, "/admin/threed?message=Result deleted successfully")
}

// AppConfigPageHandler renders the app config page
func AppConfigPageHandler(c *gin.Context) {
	var config struct {
		ID                 int
		LatestVersion      string
		MinimumVersion     string
		UpdateRequired     bool
		UpdateURL          string
		UpdateMessage      string
		MaintenanceMode    bool
		MaintenanceMessage string
		ForceUpdate        bool
		AppEnabled         bool
		CreatedAt          time.Time
		UpdatedAt          time.Time
	}

	query := `
	SELECT 
		id, latest_version, minimum_version, update_required, 
		update_url, update_message, maintenance_mode, maintenance_message,
		force_update, app_enabled, created_at, updated_at
	FROM app_config 
	ORDER BY id DESC 
	LIMIT 1
	`
	err := db.QueryRow(query).Scan(
		&config.ID,
		&config.LatestVersion,
		&config.MinimumVersion,
		&config.UpdateRequired,
		&config.UpdateURL,
		&config.UpdateMessage,
		&config.MaintenanceMode,
		&config.MaintenanceMessage,
		&config.ForceUpdate,
		&config.AppEnabled,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err != nil && err != sql.ErrNoRows {
		c.HTML(http.StatusInternalServerError, "app_config.html", gin.H{
			"error": "Failed to load config",
		})
		return
	}

	c.HTML(http.StatusOK, "app_config.html", gin.H{
		"title":  "App Configuration - Admin",
		"Config": config,
	})
}

// UpdateAppConfigHandler handles updating the app config
func UpdateAppConfigHandler(c *gin.Context) {
	latestVersion := c.PostForm("latest_version")
	minimumVersion := c.PostForm("minimum_version")
	updateURL := c.PostForm("update_url")
	updateMessage := c.PostForm("update_message")
	maintenanceMessage := c.PostForm("maintenance_message")

	updateRequired := c.PostForm("update_required") == "true"
	forceUpdate := c.PostForm("force_update") == "true"
	maintenanceMode := c.PostForm("maintenance_mode") == "true"
	appEnabled := c.PostForm("app_enabled") == "true"

	query := `
	UPDATE app_config SET
		latest_version = $1,
		minimum_version = $2,
		update_required = $3,
		update_url = $4,
		update_message = $5,
		maintenance_mode = $6,
		maintenance_message = $7,
		force_update = $8,
		app_enabled = $9,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = (SELECT id FROM app_config ORDER BY id DESC LIMIT 1)
	`

	_, err := db.Exec(
		query,
		latestVersion,
		minimumVersion,
		updateRequired,
		updateURL,
		updateMessage,
		maintenanceMode,
		maintenanceMessage,
		forceUpdate,
		appEnabled,
	)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "app_config.html", gin.H{
			"error": "Failed to update config: " + err.Error(),
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/appconfig?message=Configuration updated successfully")
}

// ServeImageHandler serves images from the uploads directory via API endpoint
// ServeImageHandler serves images via API endpoint to bypass static file restrictions
func ServeImageHandler(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename required"})
		return
	}

	// Construct file path
	filePath := filepath.Join("uploads", filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Serve the file with appropriate content type (Gin handles this automatically)
	c.File(filePath)
}
