package ws

import (
	"fmt"
	"html"
	"necore/config"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

// escapeLogText escapes dynamic text before it is placed inside an HTML log
// fragment. The dashboard currently receives logs as HTML strings, so every
// untrusted value must be escaped before it is wrapped by the colored span.
func escapeLogText(text string) string {
	return html.EscapeString(text)
}

func coloredLogMsg(color string, text string) string {
	return "<span style=\"color: " + color + ";\">" + escapeLogText(text) + "</span>"
}

func INFLogMsg(text string) string {
	return coloredLogMsg("#409EFF", text)
}

func SUCLogMsg(text string) string {
	return coloredLogMsg("#67C23A", text)
}

func WRNLogMsg(text string) string {
	return coloredLogMsg("#E6A23C", text)
}

func ERRLogMsg(text string) string {
	return coloredLogMsg("#F56C6C", text)
}

func DBGLogMsg(text string) string {
	return coloredLogMsg("#909399", text)
}

type Client struct {
	SessionID  string          `json:"session_id"`
	Identifier string          `json:"identifier"`
	TokenID    uint            `json:"token_id"`
	TokenName  string          `json:"token_name"`
	Connected  string          `json:"connected"`
	Conn       *websocket.Conn `json:"-"`
}

type Hub struct {
	Clients map[string]*Client
	mu      sync.RWMutex

	Logs  []string
	logMu sync.Mutex
}

var GlobalHub = &Hub{
	Clients: make(map[string]*Client),
	Logs:    make([]string, 0),
}

type LogLevel int

const (
	DEBUG   LogLevel = 0
	INFO    LogLevel = 1
	WARNING LogLevel = 2
	ERROR   LogLevel = 3
	SUCCESS LogLevel = 4
)

func (h *Hub) AddLog(msg string, level LogLevel) {
	BOT_LOG_BUFFER_SIZE, _ := strconv.Atoi(config.Config("BOT_LOG_BUFFER_SIZE"))
	h.logMu.Lock()
	defer h.logMu.Unlock()
	logLevelStr := ""
	switch level {
	case DEBUG:
		logLevelStr = DBGLogMsg("DBG")
	case INFO:
		logLevelStr = INFLogMsg("INF")
	case WARNING:
		logLevelStr = WRNLogMsg("WRN")
	case ERROR:
		logLevelStr = ERRLogMsg("ERR")
	case SUCCESS:
		logLevelStr = SUCLogMsg("SUC")
	}
	message := fmt.Sprintf(
		"[%v] %s | %s",
		time.Now().Format("2006-01-02 15:04:05"),
		logLevelStr,
		msg,
	)
	h.Logs = append(h.Logs, message)
	if len(h.Logs) > BOT_LOG_BUFFER_SIZE {
		h.Logs = h.Logs[:BOT_LOG_BUFFER_SIZE]
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Clients[client.SessionID] = client
	h.AddLog(
		fmt.Sprintf(
			"✅ %s 已连接，使用密钥：%s",
			WRNLogMsg(client.Identifier),
			INFLogMsg(client.TokenName),
		),
		SUCCESS,
	)
}

func (h *Hub) Unregister(sessionID, reason string, unexpected bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if client, ok := h.Clients[sessionID]; ok {
		client.Conn.Close()
		delete(h.Clients, sessionID)
		if unexpected {
			h.AddLog(
				fmt.Sprintf(
					"❌ %s 异常断开连接，原因：%s，使用密钥：%s",
					WRNLogMsg(client.Identifier),
					ERRLogMsg(reason),
					INFLogMsg(client.TokenName),
				),
				ERROR,
			)
		} else {
			h.AddLog(
				fmt.Sprintf(
					"❌ %s 断开连接，原因：%s，使用密钥：%s",
					WRNLogMsg(client.Identifier),
					ERRLogMsg(reason),
					INFLogMsg(client.TokenName),
				),
				INFO,
			)
		}
	}
}

func (h *Hub) KickByTokenID(tokenID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sessionID, client := range h.Clients {
		if client.TokenID == tokenID {
			client.Conn.Close()
			delete(h.Clients, sessionID)
			h.AddLog(
				fmt.Sprintf(
					"⚠️ %s 因为密钥删除被踢出，使用密钥：%s",
					WRNLogMsg(client.Identifier),
					INFLogMsg(client.TokenName),
				),
				WARNING,
			)
		}
	}
}

func (h *Hub) Broadcast(message interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.Clients {
		_ = client.Conn.WriteJSON(message)
	}
}

// safeClientForDashboard returns a shallow copy whose display fields are safe
// for HTML rendering. It keeps the JSON field names and types unchanged while
// avoiding mutation of the internal Client object used by the WebSocket hub.
func safeClientForDashboard(c *Client) *Client {
	if c == nil {
		return nil
	}

	copied := *c
	copied.Identifier = escapeLogText(copied.Identifier)
	copied.TokenName = escapeLogText(copied.TokenName)
	return &copied
}

func (h *Hub) GetDashboardStats() ([]*Client, []string) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.Clients))
	for _, c := range h.Clients {
		clients = append(clients, safeClientForDashboard(c))
	}
	h.mu.RUnlock()

	h.logMu.Lock()
	logsCopy := make([]string, len(h.Logs))
	copy(logsCopy, h.Logs)
	h.logMu.Unlock()

	return clients, logsCopy
}
