package timer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) events(c *gin.Context) {
	client := make(chan struct{}, 1)
	s.mu.Lock()
	if len(s.clients) >= 50 {
		s.mu.Unlock()
		writeError(c, &apiError{429, "CAPACITY", "表示接続が上限に達しました"})
		return
	}
	s.clients[client] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, client)
		s.mu.Unlock()
	}()
	c.Header("Content-Type", "text/event-stream")
	c.Header("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	controller := http.NewResponseController(c.Writer)
	first := true
	c.Stream(func(w io.Writer) bool {
		if !first {
			select {
			case <-c.Request.Context().Done():
				return false
			case <-ticker.C:
			case <-client:
			}
		}
		first = false
		s.mu.Lock()
		state := s.model.snapshot(s.now())
		s.mu.Unlock()
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		data, _ := json.Marshal(state)
		_, err := fmt.Fprintf(w, "event: state\ndata: %s\n\n", data)
		return err == nil
	})
}
