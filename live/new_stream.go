package live

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// NewLotteryDataInput represents incoming data from simplerunner (source API key format)
type NewLotteryDataInput struct {
	Date        string `json:"date"`
	Live        string `json:"live"`
	Set         string `json:"set"`
	Value       string `json:"value"`
	Status      string `json:"status"`
	UpdateTime  string `json:"updatetime"`
	Result300   string `json:"300"`
	Result430   string `json:"430"`
	Result1100  string `json:"1100"`
	Result1200  string `json:"1200"`
	Set1200     string `json:"1200set"`
	Value1200   string `json:"1200value"`
	Set430      string `json:"430set"`
	Value430    string `json:"430value"`
	Modern930   string `json:"930modern"`
	Internet930 string `json:"930internet"`
	Modern200   string `json:"200modern"`
	Internet200 string `json:"200internet"`
}

// NewLotteryData represents output data broadcast to SSE clients (clean t-prefix key format)
type NewLotteryData struct {
	Date        string `json:"date"`
	Live        string `json:"live"`
	Set         string `json:"set"`
	Value       string `json:"value"`
	Status      string `json:"status"`
	UpdateTime  string `json:"updatetime"`
	Result300   string `json:"t0300_result"`
	Result430   string `json:"t0430_result"`
	Result1100  string `json:"t1100_result"`
	Result1200  string `json:"t1200_result"`
	Set1200     string `json:"t1200_set"`
	Value1200   string `json:"t1200_val"`
	Set430      string `json:"t0430_set"`
	Value430    string `json:"t0430_val"`
	Modern930   string `json:"t0930_modern"`
	Internet930 string `json:"t0930_internet"`
	Modern200   string `json:"t0200_modern"`
	Internet200 string `json:"t0200_internet"`
	ViewCount   int    `json:"viewCount"`
}

// ToNewLotteryData converts NewLotteryDataInput to NewLotteryData
func (input *NewLotteryDataInput) ToNewLotteryData() *NewLotteryData {
	return &NewLotteryData{
		Date:        input.Date,
		Live:        input.Live,
		Set:         input.Set,
		Value:       input.Value,
		Status:      input.Status,
		UpdateTime:  input.UpdateTime,
		Result300:   input.Result300,
		Result430:   input.Result430,
		Result1100:  input.Result1100,
		Result1200:  input.Result1200,
		Set1200:     input.Set1200,
		Value1200:   input.Value1200,
		Set430:      input.Set430,
		Value430:    input.Value430,
		Modern930:   input.Modern930,
		Internet930: input.Internet930,
		Modern200:   input.Modern200,
		Internet200: input.Internet200,
		ViewCount:   0, // Set by server
	}
}

// Global state for new stream (separate from original stream)
var (
	newCurrentData  *NewLotteryData
	newDataMutex    sync.RWMutex
	newClients      = make(map[chan string]bool)
	newClientsMutex sync.RWMutex
)

// InitNew initializes the new stream with default data
func InitNew() {
	newCurrentData = &NewLotteryData{
		Live:        "--",
		Set:         "--",
		Value:       "--",
		Status:      "Off",
		Result300:   "--",
		Result430:   "--",
		Result1100:  "--",
		Result1200:  "--",
		Set1200:     "--",
		Value1200:   "--",
		Set430:      "--",
		Value430:    "--",
		Modern930:   "--",
		Internet930: "--",
		Modern200:   "--",
		Internet200: "--",
		UpdateTime:  time.Now().Format("2006/01/02 03:04:05 PM"),
	}
	log.Println("✅ New stream initialized with default data")
}

// UpdateNewLotteryData handles POST requests to update new stream data
func UpdateNewLotteryData(c *gin.Context) {
	var inputData NewLotteryDataInput

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to read request body"})
		return
	}

	if err := json.Unmarshal(body, &inputData); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	newData := inputData.ToNewLotteryData()

	newDataMutex.Lock()
	newCurrentData = newData
	newDataMutex.Unlock()

	log.Printf("📊 New stream updated - Live: %s | Status: %s | 11:00: %s | 3:00: %s",
		newData.Live, newData.Status, newData.Result1100, newData.Result300)

	broadcastNewUpdate()

	c.JSON(200, gin.H{
		"status":  "success",
		"message": "New stream data updated successfully",
		"data":    newData,
	})
}

// GetCurrentNewData returns the current new stream data
func GetCurrentNewData(c *gin.Context) {
	newDataMutex.RLock()
	data := newCurrentData
	newDataMutex.RUnlock()

	c.JSON(200, gin.H{
		"status": "success",
		"data":   data,
	})
}

// StreamNewLotteryData handles SSE streaming for the new stream (11:00 & 3:00)
func StreamNewLotteryData(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	clientChan := make(chan string, 10)

	newClientsMutex.Lock()
	newClients[clientChan] = true
	clientCount := len(newClients)
	newClientsMutex.Unlock()

	log.Printf("📡 New stream SSE client connected (Total: %d)", clientCount)

	// Send initial data immediately
	newDataMutex.Lock()
	newCurrentData.ViewCount = clientCount
	initialData, _ := json.Marshal(newCurrentData)
	newDataMutex.Unlock()

	c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", initialData)))
	c.Writer.Flush()

	notify := c.Request.Context().Done()
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-notify:
			newClientsMutex.Lock()
			delete(newClients, clientChan)
			newClientsMutex.Unlock()
			close(clientChan)
			log.Printf("📴 New stream SSE client disconnected (Remaining: %d)", len(newClients))
			return
		case <-pingTicker.C:
			c.Writer.Write([]byte(": keepalive\n\n"))
			if flusher, ok := c.Writer.(interface{ Flush() }); ok {
				flusher.Flush()
			}
		case message := <-clientChan:
			c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", message)))
			c.Writer.Flush()
		}
	}
}

// broadcastNewUpdate sends updates to all connected new stream SSE clients
func broadcastNewUpdate() {
	newDataMutex.Lock()
	newCurrentData.ViewCount = len(newClients)
	data, err := json.Marshal(newCurrentData)
	newDataMutex.Unlock()

	if err != nil {
		log.Printf("❌ Failed to marshal new stream data: %v", err)
		return
	}

	newClientsMutex.RLock()
	defer newClientsMutex.RUnlock()

	message := string(data)
	for clientChan := range newClients {
		select {
		case clientChan <- message:
		default:
			log.Println("⚠️  New stream client channel full, skipping...")
		}
	}

	log.Printf("📤 New stream broadcast to %d clients", len(newClients))
}
