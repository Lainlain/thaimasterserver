package chat

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var db *sql.DB

// InitDB receives the shared DB connection
func InitDB(database *sql.DB) {
	db = database
	log.Println("✅ Chat module initialized")
}

// ── Structs ──────────────────────────────────────────────────────────────────

type ChatUser struct {
	ID          int       `json:"id"`
	GoogleID    string    `json:"google_id,omitempty"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Role        string    `json:"role"`
	IsBanned    bool      `json:"is_banned"`
	BanReason   string    `json:"ban_reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type ChatMessage struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Message     string `json:"message"`
	CreatedAt   string `json:"created_at"`
}

// ── WebSocket upgrader ───────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// ── Google ID Token verification ─────────────────────────────────────────────

type googleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	EmailVerified string `json:"email_verified"`
	Error         string `json:"error"`
	ErrorDesc     string `json:"error_description"`
}

func verifyGoogleIDToken(idToken string) (*googleTokenInfo, error) {
	url := "https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var info googleTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse token info: %w", err)
	}
	if info.Error != "" {
		return nil, fmt.Errorf("invalid token: %s", info.ErrorDesc)
	}
	if info.Sub == "" {
		return nil, fmt.Errorf("empty sub in token")
	}
	return &info, nil
}

// ── Auth: Google Login ───────────────────────────────────────────────────────

// POST /api/chat/auth/google
// Body: { "id_token": "...", "device_id": "..." }
func GoogleLoginHandler(c *gin.Context) {
	var req struct {
		IDToken  string `json:"id_token" binding:"required"`
		DeviceID string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "id_token required"})
		return
	}

	// Check device ban first
	if req.DeviceID != "" {
		var banned bool
		db.QueryRow("SELECT EXISTS(SELECT 1 FROM chat_device_bans WHERE device_id = $1)", req.DeviceID).Scan(&banned)
		if banned {
			c.JSON(403, gin.H{"error": "device_banned", "message": "This device is banned from chat"})
			return
		}
	}

	// Verify Google token
	info, err := verifyGoogleIDToken(req.IDToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid_token", "message": err.Error()})
		return
	}

	// Upsert user
	user, err := upsertUser(info)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Check user ban
	if user.IsBanned {
		c.JSON(403, gin.H{"error": "user_banned", "message": "You are banned from chat", "ban_reason": user.BanReason})
		return
	}

	c.JSON(200, user)
}

// upsertUser creates or updates a user from Google token info
func upsertUser(info *googleTokenInfo) (*ChatUser, error) {
	var user ChatUser
	err := db.QueryRow(`
		INSERT INTO chat_users (google_id, display_name, avatar_url)
		VALUES ($1, $2, $3)
		ON CONFLICT (google_id) DO UPDATE
		SET updated_at = CURRENT_TIMESTAMP
		RETURNING id, google_id, display_name, avatar_url, role, is_banned, ban_reason, created_at
	`, info.Sub, info.Name, info.Picture).Scan(
		&user.ID, &user.GoogleID, &user.DisplayName, &user.AvatarURL,
		&user.Role, &user.IsBanned, &user.BanReason, &user.CreatedAt,
	)
	return &user, err
}

// ── Profile ──────────────────────────────────────────────────────────────────

// PUT /api/chat/profile
// Body: { "user_id": 1, "display_name": "...", "avatar_url": "..." }
func UpdateProfileHandler(c *gin.Context) {
	var req struct {
		UserID      int    `json:"user_id" binding:"required"`
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.DisplayName) == "" {
		c.JSON(400, gin.H{"error": "display_name cannot be empty"})
		return
	}

	_, err := db.Exec(`
		UPDATE chat_users
		SET display_name = $1, avatar_url = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`, req.DisplayName, req.AvatarURL, req.UserID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Profile updated"})
}

// ── Avatar Gallery ───────────────────────────────────────────────────────────

// GET /api/chat/avatars
// Returns 30 DiceBear avatar URLs (adventurer style)
func GetAvatarGalleryHandler(c *gin.Context) {
	seeds := []string{
		"alex", "sam", "riley", "jordan", "taylor",
		"morgan", "casey", "dana", "jamie", "skyler",
		"avery", "quinn", "reese", "parker", "sage",
		"river", "phoenix", "rowan", "blake", "drew",
		"charlie", "harley", "ember", "storm", "luna",
		"nova", "zephyr", "raven", "aurora", "echo",
	}

	style := c.DefaultQuery("style", "adventurer")
	var avatars []map[string]string
	for i, seed := range seeds {
		avatars = append(avatars, map[string]string{
			"id":  fmt.Sprintf("%d", i+1),
			"url": fmt.Sprintf("https://api.dicebear.com/9.x/%s/svg?seed=%s", style, seed),
		})
	}
	c.JSON(200, gin.H{"avatars": avatars, "style": style})
}

// ── Messages ─────────────────────────────────────────────────────────────────

// GET /api/chat/messages?limit=50
func GetMessagesHandler(c *gin.Context) {
	limit := 50
	rows, err := db.Query(`
		SELECT m.id, m.user_id, m.display_name, m.avatar_url, m.message, m.created_at
		FROM chat_messages m
		ORDER BY m.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		var createdAt time.Time
		if err := rows.Scan(&msg.ID, &msg.UserID, &msg.DisplayName, &msg.AvatarURL, &msg.Message, &createdAt); err != nil {
			continue
		}
		msg.CreatedAt = createdAt.Format(time.RFC3339)
		messages = append(messages, msg)
	}

	// Reverse so oldest first
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	if messages == nil {
		messages = []ChatMessage{}
	}

	c.JSON(200, gin.H{"messages": messages})
}

// ── WebSocket ────────────────────────────────────────────────────────────────

// GET /api/chat/ws?user_id=1&device_id=xxx
func WSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade failed: %v", err)
		return
	}

	userIDStr := c.Query("user_id")
	deviceID := c.Query("device_id")
	var userID int
	fmt.Sscanf(userIDStr, "%d", &userID)

	// Check device ban
	if deviceID != "" {
		var banned bool
		db.QueryRow("SELECT EXISTS(SELECT 1 FROM chat_device_bans WHERE device_id = $1)", deviceID).Scan(&banned)
		if banned {
			msg, _ := json.Marshal(OutgoingMessage{Type: "banned", Message: "This device is banned from chat"})
			conn.WriteMessage(websocket.TextMessage, msg)
			conn.Close()
			return
		}
	}

	// Check user ban
	if userID > 0 {
		var isBanned bool
		var banReason string
		db.QueryRow("SELECT is_banned, ban_reason FROM chat_users WHERE id = $1", userID).Scan(&isBanned, &banReason)
		if isBanned {
			msg, _ := json.Marshal(OutgoingMessage{Type: "banned", Message: "You are banned: " + banReason})
			conn.WriteMessage(websocket.TextMessage, msg)
			conn.Close()
			return
		}
	}

	client := &Client{
		hub:    globalHub,
		conn:   conn,
		send:   make(chan []byte, 64),
		userID: userID,
	}
	globalHub.Register(client)
	defer globalHub.Unregister(client)

	// Start write pump in background
	go client.writePump()

	// Read pump (main goroutine)
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	type IncomingMessage struct {
		UserID   int    `json:"user_id"`
		Message  string `json:"message"`
		DeviceID string `json:"device_id"`
	}

	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var incoming IncomingMessage
		if err := json.Unmarshal(rawMsg, &incoming); err != nil {
			continue
		}

		msg := strings.TrimSpace(incoming.Message)
		if msg == "" || incoming.UserID <= 0 {
			continue
		}
		// Limit message length
		if len([]rune(msg)) > 300 {
			msg = string([]rune(msg)[:300])
		}

		// Re-check user ban on every message
		var isBanned bool
		var banReason string
		db.QueryRow("SELECT is_banned, ban_reason FROM chat_users WHERE id = $1", incoming.UserID).Scan(&isBanned, &banReason)
		if isBanned {
			banMsg, _ := json.Marshal(OutgoingMessage{Type: "banned", Message: "You are banned: " + banReason})
			client.send <- banMsg
			continue
		}

		// Re-check device ban
		if incoming.DeviceID != "" {
			var devBanned bool
			db.QueryRow("SELECT EXISTS(SELECT 1 FROM chat_device_bans WHERE device_id = $1)", incoming.DeviceID).Scan(&devBanned)
			if devBanned {
				banMsg, _ := json.Marshal(OutgoingMessage{Type: "banned", Message: "This device is banned from chat"})
				client.send <- banMsg
				continue
			}
		}

		// Fetch user info
		var user ChatUser
		err = db.QueryRow("SELECT id, display_name, avatar_url FROM chat_users WHERE id = $1", incoming.UserID).
			Scan(&user.ID, &user.DisplayName, &user.AvatarURL)
		if err != nil {
			continue
		}

		// Save to DB
		var savedID int
		var savedAt time.Time
		err = db.QueryRow(`
			INSERT INTO chat_messages (user_id, display_name, avatar_url, message)
			VALUES ($1, $2, $3, $4)
			RETURNING id, created_at
		`, user.ID, user.DisplayName, user.AvatarURL, msg).Scan(&savedID, &savedAt)
		if err != nil {
			log.Printf("❌ Failed to save chat message: %v", err)
			continue
		}

		// Broadcast to all
		globalHub.Broadcast(OutgoingMessage{
			Type:        "message",
			ID:          savedID,
			UserID:      user.ID,
			DisplayName: user.DisplayName,
			AvatarURL:   user.AvatarURL,
			Message:     msg,
			CreatedAt:   savedAt.Format(time.RFC3339),
		})
	}
}

// ── Admin: Ban / Unban ───────────────────────────────────────────────────────

// POST /api/admin/chat/ban
func BanUserHandler(c *gin.Context) {
	var req struct {
		UserID    int    `json:"user_id" binding:"required"`
		BanReason string `json:"ban_reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	_, err := db.Exec(`
		UPDATE chat_users SET is_banned = TRUE, ban_reason = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, req.BanReason, req.UserID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	log.Printf("🔨 User %d banned: %s", req.UserID, req.BanReason)
	c.JSON(200, gin.H{"message": "User banned"})
}

// POST /api/admin/chat/unban
func UnbanUserHandler(c *gin.Context) {
	var req struct {
		UserID int `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	_, err := db.Exec(`
		UPDATE chat_users SET is_banned = FALSE, ban_reason = '', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, req.UserID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "User unbanned"})
}

// POST /api/admin/chat/device-ban
func BanDeviceHandler(c *gin.Context) {
	var req struct {
		DeviceID  string `json:"device_id" binding:"required"`
		BanReason string `json:"ban_reason"`
		AdminID   int    `json:"admin_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	_, err := db.Exec(`
		INSERT INTO chat_device_bans (device_id, ban_reason, banned_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (device_id) DO UPDATE SET ban_reason = EXCLUDED.ban_reason
	`, req.DeviceID, req.BanReason, req.AdminID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	log.Printf("🔨 Device banned: %s", req.DeviceID)
	c.JSON(200, gin.H{"message": "Device banned"})
}

// POST /api/admin/chat/device-unban
func UnbanDeviceHandler(c *gin.Context) {
	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	_, err := db.Exec("DELETE FROM chat_device_bans WHERE device_id = $1", req.DeviceID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Device unbanned"})
}

// POST /api/chat/report
// Body: { "reported_user_id": 1, "message_id": 42, "reason": "Spam", "reporter_device_id": "..." }
func ReportUserHandler(c *gin.Context) {
	var req struct {
		ReportedUserID   int    `json:"reported_user_id" binding:"required"`
		MessageID        *int   `json:"message_id"`
		Reason           string `json:"reason"`
		ReporterDeviceID string `json:"reporter_device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "reported_user_id is required"})
		return
	}

	// Prevent duplicate reports from same device within 1 hour
	var existingCount int
	db.QueryRow(`
		SELECT COUNT(*) FROM chat_reports
		WHERE reported_user_id = $1
		  AND reporter_device_id = $2
		  AND created_at > NOW() - INTERVAL '1 hour'
	`, req.ReportedUserID, req.ReporterDeviceID).Scan(&existingCount)
	if existingCount > 0 {
		c.JSON(200, gin.H{"message": "Already reported"})
		return
	}

	_, err := db.Exec(`
		INSERT INTO chat_reports (reported_user_id, message_id, reporter_device_id, reason)
		VALUES ($1, $2, $3, $4)
	`, req.ReportedUserID, req.MessageID, req.ReporterDeviceID, req.Reason)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to submit report"})
		return
	}

	log.Printf("🚩 User %d reported | reason: %s | device: %s", req.ReportedUserID, req.Reason, req.ReporterDeviceID)
	c.JSON(200, gin.H{"message": "Report submitted"})
}

// GET /api/admin/chat/reports
func GetReportsHandler(c *gin.Context) {
	rows, err := db.Query(`
		SELECT
			r.id, r.reported_user_id, u.display_name,
			r.message_id, r.reporter_device_id, r.reason,
			r.is_reviewed, r.created_at
		FROM chat_reports r
		JOIN chat_users u ON u.id = r.reported_user_id
		ORDER BY r.created_at DESC
		LIMIT 200
	`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type ReportRow struct {
		ID               int       `json:"id"`
		ReportedUserID   int       `json:"reported_user_id"`
		ReportedName     string    `json:"reported_name"`
		MessageID        *int      `json:"message_id"`
		ReporterDeviceID string    `json:"reporter_device_id"`
		Reason           string    `json:"reason"`
		IsReviewed       bool      `json:"is_reviewed"`
		CreatedAt        time.Time `json:"created_at"`
	}

	var reports []ReportRow
	for rows.Next() {
		var rr ReportRow
		rows.Scan(&rr.ID, &rr.ReportedUserID, &rr.ReportedName,
			&rr.MessageID, &rr.ReporterDeviceID, &rr.Reason,
			&rr.IsReviewed, &rr.CreatedAt)
		reports = append(reports, rr)
	}
	if reports == nil {
		reports = []ReportRow{}
	}
	c.JSON(200, gin.H{"reports": reports, "total": len(reports)})
}

// GET /api/admin/chat/users
func GetChatUsersHandler(c *gin.Context) {
	rows, err := db.Query(`
		SELECT id, display_name, avatar_url, role, is_banned, ban_reason, created_at
		FROM chat_users
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var users []ChatUser
	for rows.Next() {
		var u ChatUser
		rows.Scan(&u.ID, &u.DisplayName, &u.AvatarURL, &u.Role, &u.IsBanned, &u.BanReason, &u.CreatedAt)
		users = append(users, u)
	}
	if users == nil {
		users = []ChatUser{}
	}
	c.JSON(200, users)
}
