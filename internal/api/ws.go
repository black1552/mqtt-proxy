package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// 每个周期通过监控 socket 推送的负载。
type statsFrame struct {
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"ts"`
	Info      map[string]any `json:"info"`
	Clients   int            `json:"clients"`
}

// monitorWS 将实时的 Broker 事件与周期性计数器流式推送到控制台。
func (s *Server) monitorWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // 升级失败，无需再处理
	}
	defer conn.Close()

	events := s.broker.Subscribe()
	defer s.broker.Unsubscribe(events)

	done := make(chan struct{})
	ticker := time.NewTicker(s.cfg.MonitorInterval)
	defer ticker.Stop()

	// readPump 仅监听连接关闭；这里不期望来客户端命令。
	go func() {
		defer close(done)
		conn.SetReadLimit(4096)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	write := func(v any) bool {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return conn.WriteMessage(websocket.TextMessage, mustJSON(v)) == nil
	}

	// 立即发送一份初始快照。
	write(s.statsFrame())

	for {
		select {
		case <-done:
			return
		case ev := <-events:
			if !write(ev) {
				return
			}
		case <-ticker.C:
			if !write(s.statsFrame()) {
				return
			}
		}
	}
}

func (s *Server) statsFrame() statsFrame {
	return statsFrame{
		Type:      "stats",
		Timestamp: time.Now(),
		Info:      s.broker.Info(),
		Clients:   len(s.broker.ConnectedClients()),
	}
}

func mustJSON(v any) []byte {
	// broker.Event 和 statsFrame 始终可序列化；忽略错误。
	b, _ := json.Marshal(v)
	return b
}
