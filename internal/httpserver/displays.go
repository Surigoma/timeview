package httpserver

import (
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"sort"
	"time"
)

const displayTimeout = 15 * time.Second
const displayRetention = 30 * time.Minute

type displayReport struct {
	ID       string `json:"id"`
	LastSeen int64  `json:"lastSeenMs"`
	Version  int64  `json:"version"`
	Online   bool   `json:"online"`
}

func (s *Server) pruneDisplays(now time.Time) {
	for id, display := range s.displays {
		if now.UnixMilli()-display.LastSeen >= displayRetention.Milliseconds() {
			delete(s.displays, id)
		}
	}
}

// Caller holds s.mu. A snapshot includes timeout transitions without a browser poll.
func (s *Server) displaySnapshot(now time.Time) []displayReport {
	s.pruneDisplays(now)
	displays := make([]displayReport, 0, len(s.displays))
	for _, display := range s.displays {
		display.Online = now.UnixMilli()-display.LastSeen < displayTimeout.Milliseconds()
		displays = append(displays, display)
	}
	sort.Slice(displays, func(i, j int) bool { return displays[i].ID < displays[j].ID })
	return displays
}

func (s *Server) getDisplays(c *gin.Context) {
	s.mu.Lock()
	displays := s.displaySnapshot(s.now())
	instance := s.model.InstanceID
	s.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"instanceId": instance, "displays": displays})
}

// Reports acknowledge a browser that has received timer state, not physical screen output.
// They are idempotent telemetry and never mutate or persist the timer model.
func (s *Server) reportDisplay(c *gin.Context) {
	if !isBrowserMutation(c) {
		writeError(c, &apiError{Status: 403, Code: "BROWSER_ONLY", Message: "演台の応答は同じオリジンのブラウザから送信してください"})
		return
	}
	if c.ContentType() != "application/json" {
		writeError(c, &apiError{Status: 400, Code: "INVALID_CONTENT_TYPE", Message: "Content-Typeはapplication/jsonにしてください"})
		return
	}
	id := c.Param("id")
	if _, err := hex.DecodeString(id); err != nil || len(id) != 32 {
		writeError(c, invalid("演台IDは32桁の16進数にしてください"))
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, bodyLimit))
	if err != nil {
		writeError(c, &apiError{Status: 413, Code: "BODY_TOO_LARGE", Message: "本文は16KiB以内にしてください"})
		return
	}
	var payload struct {
		InstanceID string `json:"instanceId"`
		Version    *int64 `json:"version"`
	}
	if err := decode(data, &payload); err != nil {
		writeError(c, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if payload.InstanceID != s.model.InstanceID {
		writeError(c, conflict("サーバーが再起動しました。状態を再取得してください"))
		return
	}
	if payload.Version == nil || *payload.Version < 0 || *payload.Version > s.model.Version {
		writeError(c, invalid("受信済みの状態バージョンを指定してください"))
		return
	}
	now := s.now()
	s.pruneDisplays(now)
	if s.displays == nil {
		s.displays = make(map[string]displayReport)
	}
	previous, exists := s.displays[id]
	if !exists && len(s.displays) >= 50 {
		// Keep recent disconnected reports, but let a new display replace the oldest offline one.
		oldest := ""
		for key, display := range s.displays {
			if now.UnixMilli()-display.LastSeen >= displayTimeout.Milliseconds() && (oldest == "" || display.LastSeen < s.displays[oldest].LastSeen) {
				oldest = key
			}
		}
		if oldest == "" {
			writeError(c, &apiError{Status: 429, Code: "CAPACITY", Message: "演台接続の記録が上限に達しました"})
			return
		}
		delete(s.displays, oldest)
	}
	version := *payload.Version
	if exists && previous.Version > version {
		version = previous.Version
	}
	s.displays[id] = displayReport{ID: id, LastSeen: now.UnixMilli(), Version: version}
	s.notifyClients()
	c.Status(http.StatusNoContent)
}
