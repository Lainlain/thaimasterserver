package paper

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var db *sql.DB

type PaperType struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	DisplayOrder int       `json:"display_order"`
	IsActive     bool      `json:"is_active"`
	ImageCount   int       `json:"image_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PaperImage struct {
	ID           int       `json:"id"`
	TypeID       int       `json:"type_id"`
	TypeName     string    `json:"type_name,omitempty"`
	ImageURL     string    `json:"image_url"`
	DisplayOrder int       `json:"display_order"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PaperTypeWithImages struct {
	Type   PaperType    `json:"type"`
	Images []PaperImage `json:"images"`
}

func InitDB(database *sql.DB) {
	db = database
}

// Get all paper types with image count
func GetAllTypes(c *gin.Context) {
	rows, err := db.Query(`
		SELECT pt.id, pt.name, pt.display_order, pt.is_active, pt.created_at, pt.updated_at,
		       COALESCE(COUNT(pi.id), 0) as image_count
		FROM paper_types pt
		LEFT JOIN paper_images pi ON pt.id = pi.type_id AND pi.is_active = true
		WHERE pt.is_active = true
		GROUP BY pt.id, pt.name, pt.display_order, pt.is_active, pt.created_at, pt.updated_at
		ORDER BY pt.display_order ASC, pt.name ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var types []PaperType
	for rows.Next() {
		var t PaperType
		if err := rows.Scan(&t.ID, &t.Name, &t.DisplayOrder, &t.IsActive, &t.CreatedAt, &t.UpdatedAt, &t.ImageCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		types = append(types, t)
	}

	c.JSON(http.StatusOK, types)
}

// Get all paper types with their images (for admin)
func GetAllTypesWithImages(c *gin.Context) {
	// Get all types
	typeRows, err := db.Query(`
		SELECT id, name, display_order, is_active, created_at, updated_at
		FROM paper_types
		ORDER BY display_order ASC, name ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer typeRows.Close()

	var results []PaperTypeWithImages
	for typeRows.Next() {
		var t PaperType
		if err := typeRows.Scan(&t.ID, &t.Name, &t.DisplayOrder, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Get images for this type
		imageRows, err := db.Query(`
			SELECT id, type_id, image_url, display_order, is_active, created_at, updated_at
			FROM paper_images
			WHERE type_id = $1
			ORDER BY display_order ASC, created_at DESC
		`, t.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var images []PaperImage
		for imageRows.Next() {
			var img PaperImage
			if err := imageRows.Scan(&img.ID, &img.TypeID, &img.ImageURL, &img.DisplayOrder, &img.IsActive, &img.CreatedAt, &img.UpdatedAt); err != nil {
				imageRows.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			images = append(images, img)
		}
		imageRows.Close()

		results = append(results, PaperTypeWithImages{
			Type:   t,
			Images: images,
		})
	}

	c.JSON(http.StatusOK, results)
}

// Get images by type ID
func GetImagesByType(c *gin.Context) {
	typeID := c.Param("type_id")

	rows, err := db.Query(`
		SELECT pi.id, pi.type_id, pt.name, pi.image_url, pi.display_order, pi.is_active, pi.created_at, pi.updated_at
		FROM paper_images pi
		JOIN paper_types pt ON pi.type_id = pt.id
		WHERE pi.type_id = $1 AND pi.is_active = true AND pt.is_active = true
		ORDER BY pi.display_order ASC, pi.created_at DESC
	`, typeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var images []PaperImage
	for rows.Next() {
		var img PaperImage
		if err := rows.Scan(&img.ID, &img.TypeID, &img.TypeName, &img.ImageURL, &img.DisplayOrder, &img.IsActive, &img.CreatedAt, &img.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		images = append(images, img)
	}

	c.JSON(http.StatusOK, images)
}

// Create paper type
func CreateType(c *gin.Context) {
	var input struct {
		Name         string `json:"name" binding:"required"`
		DisplayOrder int    `json:"display_order"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id int
	err := db.QueryRow(`
		INSERT INTO paper_types (name, display_order, is_active, created_at, updated_at)
		VALUES ($1, $2, true, NOW(), NOW())
		RETURNING id
	`, input.Name, input.DisplayOrder).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "Paper type created successfully"})
}

// Update paper type
func UpdateType(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Name         string `json:"name"`
		DisplayOrder int    `json:"display_order"`
		IsActive     bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.Exec(`
		UPDATE paper_types
		SET name = $1, display_order = $2, is_active = $3, updated_at = NOW()
		WHERE id = $4
	`, input.Name, input.DisplayOrder, input.IsActive, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Paper type updated successfully"})
}

// Delete paper type
func DeleteType(c *gin.Context) {
	id := c.Param("id")

	_, err := db.Exec("DELETE FROM paper_types WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Paper type deleted successfully"})
}

// Create paper image
func CreateImage(c *gin.Context) {
	var input struct {
		TypeID       int    `json:"type_id" binding:"required"`
		ImageURL     string `json:"image_url" binding:"required"`
		DisplayOrder int    `json:"display_order"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id int
	err := db.QueryRow(`
		INSERT INTO paper_images (type_id, image_url, display_order, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, true, NOW(), NOW())
		RETURNING id
	`, input.TypeID, input.ImageURL, input.DisplayOrder).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "Paper image created successfully"})
}

// Update paper image
func UpdateImage(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		TypeID       int    `json:"type_id"`
		ImageURL     string `json:"image_url"`
		DisplayOrder int    `json:"display_order"`
		IsActive     bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.Exec(`
		UPDATE paper_images
		SET type_id = $1, image_url = $2, display_order = $3, is_active = $4, updated_at = NOW()
		WHERE id = $5
	`, input.TypeID, input.ImageURL, input.DisplayOrder, input.IsActive, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Paper image updated successfully"})
}

// Delete paper image
func DeleteImage(c *gin.Context) {
	id := c.Param("id")

	_, err := db.Exec("DELETE FROM paper_images WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Paper image deleted successfully"})
}

// Batch create images
func BatchCreateImages(c *gin.Context) {
	var input struct {
		TypeID    int      `json:"type_id" binding:"required"`
		ImageURLs []string `json:"image_urls" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var insertedIDs []int
	for i, url := range input.ImageURLs {
		var id int
		err := tx.QueryRow(`
			INSERT INTO paper_images (type_id, image_url, display_order, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, true, NOW(), NOW())
			RETURNING id
		`, input.TypeID, url, i).Scan(&id)

		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		insertedIDs = append(insertedIDs, id)
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Images created successfully",
		"count":   len(insertedIDs),
		"ids":     insertedIDs,
	})
}

// Get type by ID
func GetTypeByID(id string) (*PaperType, error) {
	var t PaperType
	err := db.QueryRow(`
		SELECT id, name, display_order, is_active, created_at, updated_at
		FROM paper_types
		WHERE id = $1
	`, id).Scan(&t.ID, &t.Name, &t.DisplayOrder, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		return nil, err
	}

	// Get image count
	db.QueryRow(`
		SELECT COUNT(*) FROM paper_images WHERE type_id = $1 AND is_active = true
	`, id).Scan(&t.ImageCount)

	return &t, nil
}

// Get image by ID
func GetImageByID(id string) (*PaperImage, error) {
	var img PaperImage
	err := db.QueryRow(`
		SELECT pi.id, pi.type_id, pt.name, pi.image_url, pi.display_order, pi.is_active, pi.created_at, pi.updated_at
		FROM paper_images pi
		JOIN paper_types pt ON pi.type_id = pt.id
		WHERE pi.id = $1
	`, id).Scan(&img.ID, &img.TypeID, &img.TypeName, &img.ImageURL, &img.DisplayOrder, &img.IsActive, &img.CreatedAt, &img.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return &img, nil
}

// Get next display order for type
func GetNextDisplayOrder(typeID int) int {
	var order int
	err := db.QueryRow(`
		SELECT COALESCE(MAX(display_order), 0) + 1
		FROM paper_images
		WHERE type_id = $1
	`, typeID).Scan(&order)

	if err != nil {
		return 1
	}
	return order
}
