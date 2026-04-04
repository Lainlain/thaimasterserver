package twodhistory

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// TwoDHistory represents a single lottery history record
type TwoDHistory struct {
	ID          int       `json:"id,omitempty" db:"id"`
	Date        string    `json:"date" db:"date"`
	Set1200     string    `json:"1200set" db:"set1200"`
	Value1200   string    `json:"1200value" db:"value1200"`
	Result1200  string    `json:"1200" db:"result1200"`
	Set430      string    `json:"430set" db:"set430"`
	Value430    string    `json:"430value" db:"value430"`
	Result430   string    `json:"430" db:"result430"`
	Modern930   string    `json:"930modern" db:"modern930"`
	Internet930 string    `json:"930internet" db:"internet930"`
	Modern200   string    `json:"200modern" db:"modern200"`
	Internet200 string    `json:"200internet" db:"internet200"`
	CreatedAt   time.Time `json:"created_at,omitempty" db:"created_at"`
}

var db *sql.DB

// LotteryData represents incoming lottery data (to avoid circular dependency)
type LotteryData struct {
	Date        string `json:"date"`
	Live        string `json:"live"`
	Status      string `json:"status"`
	Set1200     string `json:"1200set"`
	Value1200   string `json:"1200value"`
	Result1200  string `json:"1200"`
	Set430      string `json:"430set"`
	Value430    string `json:"430value"`
	Result430   string `json:"430"`
	Modern930   string `json:"930modern"`
	Internet930 string `json:"930internet"`
	Modern200   string `json:"200modern"`
	Internet200 string `json:"200internet"`
	UpdateTime  string `json:"updatetime"`
}

// InitDB receives the shared database connection (opened and migrated by db.Connect)
func InitDB(database *sql.DB) {
	db = database
	log.Println("✅ twodhistory module initialized")
}

// InsertHistory inserts a new history record if the date doesn't exist
func InsertHistory(history *TwoDHistory) error {
	// Check if date already exists
	exists, err := DateExists(history.Date)
	if err != nil {
		return err
	}

	if exists {
		log.Printf("⚠️  History for date %s already exists, skipping insert", history.Date)
		return nil
	}

	query := `
	INSERT INTO twodhistory (
		date, set1200, value1200, result1200,
		set430, value430, result430,
		modern930, internet930, modern200, internet200
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = db.Exec(query,
		history.Date,
		history.Set1200,
		history.Value1200,
		history.Result1200,
		history.Set430,
		history.Value430,
		history.Result430,
		history.Modern930,
		history.Internet930,
		history.Modern200,
		history.Internet200,
	)

	if err != nil {
		return fmt.Errorf("failed to insert history: %w", err)
	}

	log.Printf("✅ Inserted history for date: %s", history.Date)
	return nil
}

// InsertFromLotteryData inserts history from LotteryData struct
func InsertFromLotteryData(data *LotteryData) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	history := &TwoDHistory{
		Date:        data.Date,
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
	}

	return InsertHistory(history)
}

// DateExists checks if a history record for the given date already exists
func DateExists(date string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM twodhistory WHERE date = $1"
	err := db.QueryRow(query, date).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check date existence: %w", err)
	}
	return count > 0, nil
}

// GetAllHistory retrieves all history records ordered by date DESC
func GetAllHistory() ([]TwoDHistory, error) {
	query := `
	SELECT id, date, set1200, value1200, result1200,
	       set430, value430, result430,
	       modern930, internet930, modern200, internet200,
	       created_at
	FROM twodhistory
	ORDER BY date DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var histories []TwoDHistory
	for rows.Next() {
		var h TwoDHistory
		err := rows.Scan(
			&h.ID, &h.Date, &h.Set1200, &h.Value1200, &h.Result1200,
			&h.Set430, &h.Value430, &h.Result430,
			&h.Modern930, &h.Internet930, &h.Modern200, &h.Internet200,
			&h.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		histories = append(histories, h)
	}

	return histories, nil
}

// GetHistoryHandler is the Gin handler for GET /api/twodhistory
func GetHistoryHandler(c *gin.Context) {
	histories, err := GetAllHistory()
	if err != nil {
		log.Printf("❌ Error fetching history: %v", err)
		c.JSON(500, gin.H{"error": "Failed to fetch history"})
		return
	}

	c.JSON(200, histories)
}

// CheckAndInsertHandler is the Gin handler for POST /api/twodhistory/check
// It checks if the date exists and inserts if not
func CheckAndInsertHandler(c *gin.Context) {
	var history TwoDHistory

	if err := c.BindJSON(&history); err != nil {
		log.Printf("❌ Error binding JSON: %v", err)
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	// Insert history (will skip if date already exists)
	if err := InsertHistory(&history); err != nil {
		log.Printf("❌ Error inserting history: %v", err)
		c.JSON(500, gin.H{"error": "Failed to insert history"})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "History checked/inserted successfully",
		"date":    history.Date,
	})
}

// AdminGetAllHistoryHandler handles GET /api/admin/twodhistory
func AdminGetAllHistoryHandler(c *gin.Context) {
	histories, err := GetAllHistory()
	if err != nil {
		log.Printf("❌ Error fetching history: %v", err)
		c.JSON(500, gin.H{"error": "Failed to fetch history"})
		return
	}
	c.JSON(200, histories)
}

// AdminCreateHistoryHandler handles POST /api/admin/twodhistory
func AdminCreateHistoryHandler(c *gin.Context) {
	var history TwoDHistory
	if err := c.ShouldBindJSON(&history); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if history.Date == "" {
		c.JSON(400, gin.H{"error": "date is required"})
		return
	}

	exists, err := DateExists(history.Date)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}
	if exists {
		c.JSON(409, gin.H{"error": "History for date " + history.Date + " already exists"})
		return
	}

	var id int
	err = db.QueryRow(`
		INSERT INTO twodhistory (
			date, set1200, value1200, result1200,
			set430, value430, result430,
			modern930, internet930, modern200, internet200
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`,
		history.Date,
		history.Set1200, history.Value1200, history.Result1200,
		history.Set430, history.Value430, history.Result430,
		history.Modern930, history.Internet930,
		history.Modern200, history.Internet200,
	).Scan(&id)

	if err != nil {
		log.Printf("❌ Error inserting history: %v", err)
		c.JSON(500, gin.H{"error": "Failed to insert history"})
		return
	}

	c.JSON(201, gin.H{"id": id, "message": "History record created successfully"})
}

// AdminUpdateHistoryHandler handles PUT /api/admin/twodhistory/:id
func AdminUpdateHistoryHandler(c *gin.Context) {
	id := c.Param("id")
	var history TwoDHistory
	if err := c.ShouldBindJSON(&history); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	result, err := db.Exec(`
		UPDATE twodhistory SET
			date = $1,
			set1200 = $2, value1200 = $3, result1200 = $4,
			set430 = $5, value430 = $6, result430 = $7,
			modern930 = $8, internet930 = $9,
			modern200 = $10, internet200 = $11
		WHERE id = $12
	`,
		history.Date,
		history.Set1200, history.Value1200, history.Result1200,
		history.Set430, history.Value430, history.Result430,
		history.Modern930, history.Internet930,
		history.Modern200, history.Internet200,
		id,
	)
	if err != nil {
		log.Printf("❌ Error updating history: %v", err)
		c.JSON(500, gin.H{"error": "Failed to update history"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(404, gin.H{"error": "Record not found"})
		return
	}

	c.JSON(200, gin.H{"message": "History record updated successfully"})
}

// AdminDeleteHistoryHandler handles DELETE /api/admin/twodhistory/:id
func AdminDeleteHistoryHandler(c *gin.Context) {
	id := c.Param("id")

	result, err := db.Exec("DELETE FROM twodhistory WHERE id = $1", id)
	if err != nil {
		log.Printf("❌ Error deleting history: %v", err)
		c.JSON(500, gin.H{"error": "Failed to delete history"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(404, gin.H{"error": "Record not found"})
		return
	}

	c.JSON(200, gin.H{"message": "History record deleted successfully"})
}

// CloseDB closes the database connection
func CloseDB() {
	if db != nil {
		db.Close()
		log.Println("Database connection closed")
	}
}

// GetDB returns the database connection
func GetDB() *sql.DB {
	return db
}
