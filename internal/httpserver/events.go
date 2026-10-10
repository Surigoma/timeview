package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"timeview/internal/auditlog"
)

// Caller holds s.mu. Coalescing keeps slow consumers from blocking mutations.
func (s *Server) notifyClients() {
	for client := range s.clients {
		select {
		case client <- struct{}{}:
		default:
		}
	}
}

func (s *Server) events(c *gin.Context) {
	topics := map[string]bool{}
	if requested := c.Query("topics"); requested != "" {
		for _, topic := range strings.Split(requested, ",") {
			if topic != "displays" && topic != "logs" {
				writeError(c, invalid("配信対象はdisplaysまたはlogsを指定してください"))
				return
			}
			topics[topic] = true
		}
	}
	client := make(chan struct{}, 1)
	s.mu.Lock()
	if len(s.clients) >= 50 {
		s.mu.Unlock()
		writeError(c, &apiError{Status: 429, Code: "CAPACITY", Message: "表示接続が上限に達しました"})
		return
	}
	s.clients[client] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.clients, client); s.mu.Unlock() }()
	c.Header("Content-Type", "text/event-stream")
	c.Header("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	controller := http.NewResponseController(c.Writer)
	first := true
	var previousDisplays []byte
	var previousLogRevision uint64
	logsSent := false
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
		state := s.model.Snapshot(s.now())
		var displays []displayReport
		if topics["displays"] {
			displays = s.displaySnapshot(s.now())
		}
		log, revision := s.audit, s.logRevision
		s.mu.Unlock()
		emit := func(event string, data []byte) bool {
			_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			return err == nil
		}
		data, _ := json.Marshal(state)
		if !emit("state", data) {
			return false
		}
		if topics["displays"] {
			data, _ = json.Marshal(gin.H{"instanceId": state.InstanceID, "displays": displays})
			if !bytes.Equal(data, previousDisplays) {
				if !emit("displays", data) {
					return false
				}
				previousDisplays = data
			}
		}
		if topics["logs"] && (!logsSent || previousLogRevision != revision) {
			entries, err := s.streamLogs(log, revision)
			payload := gin.H{"instanceId": state.InstanceID, "entries": entries}
			if err != nil {
				payload["error"] = "操作ログを取得できませんでした"
			}
			data, _ = json.Marshal(payload)
			if !emit("logs", data) {
				return false
			}
			// A failed read is retried on the next heartbeat, without marking stale logs fresh.
			logsSent = err == nil
			previousLogRevision = revision
		}
		return true
	})
}

// Share each successful log snapshot among subscribers instead of rereading the
// entire audit file for every client. Published slices are never mutated.
func (s *Server) streamLogs(log *auditlog.Log, revision uint64) ([]auditlog.Entry, error) {
	s.logReadMu.Lock()
	defer s.logReadMu.Unlock()
	s.mu.Lock()
	if s.logCache != nil && s.logCacheRevision == revision {
		entries := s.logCache
		s.mu.Unlock()
		return entries, nil
	}
	s.mu.Unlock()
	entries, err := readLogs(log)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.logRevision == revision && s.audit == log {
		s.logCache, s.logCacheRevision = entries, revision
	}
	s.mu.Unlock()
	return entries, nil
}
