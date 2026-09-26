package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"golang.org/x/time/rate"

	"timeview/internal/timer"
)

type apiError = timer.APIError

func invalid(message string) error  { return timer.Invalid(message) }
func conflict(message string) error { return timer.Conflict(message) }

type clientLimit struct {
	write *rate.Limiter
	read  *rate.Limiter
	last  time.Time
}

func (s *Server) apiHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	}
}

func (s *Server) rateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		if host == "" {
			host = c.Request.RemoteAddr
		}
		now := s.now()
		s.mu.Lock()
		for key, limit := range s.limits {
			if now.Sub(limit.last) > time.Minute {
				delete(s.limits, key)
			}
		}
		limit := s.limits[host]
		if limit == nil && len(s.limits) < 1024 {
			limit = &clientLimit{
				write: rate.NewLimiter(rate.Limit(10), 20),
				read:  rate.NewLimiter(rate.Limit(30), 30),
				last:  now,
			}
			s.limits[host] = limit
		}
		allowed := limit != nil
		if allowed {
			limit.last = now
			limiter := limit.write
			if c.Request.Method == http.MethodGet {
				limiter = limit.read
			}
			allowed = limiter.AllowN(now, 1)
		}
		s.mu.Unlock()
		if !allowed {
			writeError(c, &apiError{Status: 429, Code: "RATE_LIMITED", Message: "操作間隔を空けて再試行してください"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func writeError(c *gin.Context, err error) {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		apiErr = &apiError{Status: 500, Code: "INTERNAL_ERROR", Message: "内部エラーが発生しました"}
	}
	if apiErr.Status == http.StatusTooManyRequests {
		c.Header("Retry-After", "1")
	}
	c.AbortWithStatusJSON(apiErr.Status, gin.H{"error": gin.H{"code": apiErr.Code, "message": apiErr.Message}})
}

func decode(data []byte, value any) error {
	if !utf8.Valid(data) {
		return invalid("UTF-8で送信してください")
	}
	var fields map[string]json.RawMessage
	if err := binding.JSON.BindBody(data, &fields); err != nil || fields == nil {
		return &apiError{Status: 400, Code: "INVALID_JSON", Message: "JSONオブジェクトを指定してください"}
	}
	for _, field := range fields {
		if bytes.Equal(bytes.TrimSpace(field), []byte("null")) {
			return invalid("nullは指定できません")
		}
	}
	if err := binding.JSON.BindBody(data, value); err != nil {
		return invalid("フィールド名または値の型が不正です")
	}
	return nil
}
