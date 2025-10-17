package appconfig

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var db *sql.DB

// AppConfig represents the app configuration
type AppConfig struct {
	ID                 int       `json:"id"`
	LatestVersion      string    `json:"latest_version"`
	MinimumVersion     string    `json:"minimum_version"`
	UpdateRequired     bool      `json:"update_required"`
	UpdateURL          string    `json:"update_url"`
	UpdateMessage      string    `json:"update_message"`
	MaintenanceMode    bool      `json:"maintenance_mode"`
	MaintenanceMessage string    `json:"maintenance_message"`
	ForceUpdate        bool      `json:"force_update"`
	AppEnabled         bool      `json:"app_enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// InitDB initializes the database connection for app config
func InitDB(database *sql.DB) {
	db = database
	createTable()
	insertDefaultConfig()
}

// Create app_config table if it doesn't exist
func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS app_config (
		id SERIAL PRIMARY KEY,
		latest_version VARCHAR(20) NOT NULL DEFAULT '1.0.0',
		minimum_version VARCHAR(20) NOT NULL DEFAULT '1.0.0',
		update_required BOOLEAN DEFAULT FALSE,
		update_url TEXT DEFAULT '',
		update_message TEXT DEFAULT 'A new version is available!',
		maintenance_mode BOOLEAN DEFAULT FALSE,
		maintenance_message TEXT DEFAULT 'App is under maintenance. Please try again later.',
		force_update BOOLEAN DEFAULT FALSE,
		app_enabled BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to create app_config table: %v", err)
	}
	log.Println("✅ app_config table ready")
}

// Insert default config if table is empty
func insertDefaultConfig() {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM app_config").Scan(&count)
	if err != nil {
		log.Printf("Error checking app_config: %v", err)
		return
	}

	if count == 0 {
		query := `
		INSERT INTO app_config (
			latest_version, 
			minimum_version, 
			update_required, 
			update_url,
			update_message,
			maintenance_mode,
			maintenance_message,
			force_update,
			app_enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`
		_, err = db.Exec(
			query,
			"1.0.0",                                                  // latest_version
			"1.0.0",                                                  // minimum_version
			false,                                                    // update_required
			"https://play.google.com/store/apps/details?id=com.thaimaster2d", // update_url
			"🎉 New version available! Update now for better experience.",    // update_message
			false, // maintenance_mode
			"🔧 App is under maintenance. Please check back soon!",    // maintenance_message
			false, // force_update
			true,  // app_enabled
		)
		if err != nil {
			log.Printf("Failed to insert default app_config: %v", err)
		} else {
			log.Println("✅ Default app config inserted")
		}
	}
}

// GetAppConfig returns the current app configuration
func GetAppConfig(c *gin.Context) {
	var config AppConfig
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

	if err != nil {
		log.Printf("Error fetching app config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch app config"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// CheckVersion checks if the client version is compatible
func CheckVersion(c *gin.Context) {
	clientVersion := c.Query("version")
	if clientVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version parameter required"})
		return
	}

	var config AppConfig
	query := `
	SELECT 
		latest_version, minimum_version, update_required, 
		update_url, update_message, maintenance_mode, maintenance_message,
		force_update, app_enabled
	FROM app_config 
	ORDER BY id DESC 
	LIMIT 1
	`
	err := db.QueryRow(query).Scan(
		&config.LatestVersion,
		&config.MinimumVersion,
		&config.UpdateRequired,
		&config.UpdateURL,
		&config.UpdateMessage,
		&config.MaintenanceMode,
		&config.MaintenanceMessage,
		&config.ForceUpdate,
		&config.AppEnabled,
	)

	if err != nil {
		log.Printf("Error fetching app config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check version"})
		return
	}

	// Check if app is enabled
	if !config.AppEnabled {
		c.JSON(http.StatusOK, gin.H{
			"can_use":          false,
			"message":          "App is temporarily disabled",
			"maintenance_mode": true,
		})
		return
	}

	// Check maintenance mode
	if config.MaintenanceMode {
		c.JSON(http.StatusOK, gin.H{
			"can_use":          false,
			"message":          config.MaintenanceMessage,
			"maintenance_mode": true,
		})
		return
	}

	// Compare versions
	needsUpdate := compareVersions(clientVersion, config.MinimumVersion) < 0
	hasUpdate := compareVersions(clientVersion, config.LatestVersion) < 0

	response := gin.H{
		"can_use":          !needsUpdate,
		"current_version":  clientVersion,
		"latest_version":   config.LatestVersion,
		"minimum_version":  config.MinimumVersion,
		"needs_update":     needsUpdate,
		"has_update":       hasUpdate,
		"force_update":     config.ForceUpdate && needsUpdate,
		"update_url":       config.UpdateURL,
		"update_message":   config.UpdateMessage,
		"maintenance_mode": false,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateAppConfig updates the app configuration (admin only)
func UpdateAppConfig(c *gin.Context) {
	var input AppConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
	RETURNING id
	`

	var id int
	err := db.QueryRow(
		query,
		input.LatestVersion,
		input.MinimumVersion,
		input.UpdateRequired,
		input.UpdateURL,
		input.UpdateMessage,
		input.MaintenanceMode,
		input.MaintenanceMessage,
		input.ForceUpdate,
		input.AppEnabled,
	).Scan(&id)

	if err != nil {
		log.Printf("Error updating app config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "App config updated successfully",
		"id":      id,
	})
}

// Simple version comparison (major.minor.patch)
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Simple string comparison for now
	// For production, use proper semantic versioning library
	if v1 < v2 {
		return -1
	} else if v1 > v2 {
		return 1
	}
	return 0
}
