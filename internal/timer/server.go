package timer

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"golang.org/x/time/rate"
)

const bodyLimit = 16 << 10

var configureGin sync.Once

type receipt struct {
	hash    [32]byte
	body    []byte
	etag    string
	expires time.Time
}

type clientLimit struct {
	write *rate.Limiter
	read  *rate.Limiter
	last  time.Time
}

type Server struct {
	mu         sync.Mutex
	model      model
	configPath string
	receipts   map[string]receipt
	clients    map[chan struct{}]struct{}
	limits     map[string]*clientLimit
	now        func() time.Time
	handler    http.Handler
	handlerOne sync.Once
}

func prepareGin() {
	configureGin.Do(func() {
		gin.SetMode(gin.ReleaseMode)
		gin.EnableJsonDecoderDisallowUnknownFields()
	})
}

func New() *Server {
	prepareGin()
	return &Server{model: newModel(time.Now()), receipts: map[string]receipt{}, clients: map[chan struct{}]struct{}{}, limits: map[string]*clientLimit{}, now: time.Now}
}

func NewWithConfig(path string) (*Server, error) {
	prepareGin()
	loaded, err := loadModel(path, time.Now())
	if err != nil {
		return nil, err
	}
	return &Server{model: loaded, configPath: path, receipts: map[string]receipt{}, clients: map[chan struct{}]struct{}{}, limits: map[string]*clientLimit{}, now: time.Now}, nil
}

// Register mounts the TimeView API under the supplied Gin router group.
func (s *Server) Register(api *gin.RouterGroup) {
	api.Use(s.apiHeaders(), s.rateLimit())
	api.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	api.GET("/timer", s.getTimer)
	api.GET("/timer/events", s.events)
	api.GET("/timer/presets", s.getPresets)
	api.GET("/timer/bindings", s.getBindings)
	api.PATCH("/timer", s.mutation("/timer"))
	api.POST("/timer/commands", s.mutation("/timer/commands"))
	api.PUT("/timer/blackout", s.mutation("/timer/blackout"))
	api.PUT("/timer/message", s.mutation("/timer/message"))
	api.DELETE("/timer/message", s.mutation("/timer/message"))
	api.PUT("/timer/presets", s.mutation("/timer/presets"))
	api.PUT("/timer/bindings", s.mutation("/timer/bindings"))
}

// ServeHTTP keeps Server usable with httptest and as a standalone net/http handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handlerOne.Do(func() {
		router := gin.New()
		router.Use(gin.Recovery())
		_ = router.SetTrustedProxies(nil)
		s.Register(router.Group("/api/v1"))
		router.NoRoute(func(c *gin.Context) {
			writeError(c, &apiError{404, "NOT_FOUND", "APIが見つかりません"})
		})
		s.handler = router
	})
	s.handler.ServeHTTP(w, r)
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
			writeError(c, &apiError{429, "RATE_LIMITED", "操作間隔を空けて再試行してください"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func writeError(c *gin.Context, err error) {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		apiErr = &apiError{500, "INTERNAL_ERROR", "内部エラーが発生しました"}
	}
	if apiErr.status == http.StatusTooManyRequests {
		c.Header("Retry-After", "1")
	}
	c.AbortWithStatusJSON(apiErr.status, gin.H{"error": gin.H{"code": apiErr.code, "message": apiErr.message}})
}

func decode(data []byte, value any) error {
	if !utf8.Valid(data) {
		return invalid("UTF-8で送信してください")
	}
	var fields map[string]json.RawMessage
	if err := binding.JSON.BindBody(data, &fields); err != nil || fields == nil {
		return &apiError{400, "INVALID_JSON", "JSONオブジェクトを指定してください"}
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

func (s *Server) getTimer(c *gin.Context) {
	s.mu.Lock()
	state := s.model.snapshot(s.now())
	s.mu.Unlock()
	c.Header("ETag", state.etag())
	c.JSON(http.StatusOK, state)
}

func (s *Server) getPresets(c *gin.Context) {
	s.mu.Lock()
	presets := append([]string{}, s.model.presets...)
	s.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"presets": presets})
}

func (s *Server) getBindings(c *gin.Context) {
	s.mu.Lock()
	bindings := append([]KeyBinding{}, s.model.bindings...)
	s.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"bindings": bindings})
}

func (s *Server) mutation(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodDelete && c.ContentType() != "application/json" {
			writeError(c, &apiError{400, "INVALID_CONTENT_TYPE", "Content-Typeはapplication/jsonにしてください"})
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, bodyLimit))
		if err != nil {
			writeError(c, &apiError{413, "BODY_TOO_LARGE", "本文は16KiB以内にしてください"})
			return
		}
		body, etag, err := s.mutate(c, path, data)
		if err != nil {
			writeError(c, err)
			return
		}
		log.Printf("%s %s ok", c.Request.Method, path)
		c.Header("ETag", etag)
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	}
}

func (s *Server) mutate(c *gin.Context, path string, data []byte) ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if boot := c.GetHeader("X-Timeview-Instance"); boot != "" && boot != s.model.InstanceID {
		return nil, "", conflict("サーバーが再起動しました。状態を再取得してください")
	}
	key := c.GetHeader("Idempotency-Key")
	if len(key) > 128 {
		return nil, "", invalid("Idempotency-Keyは128文字以内です")
	}
	hash := sha256.Sum256([]byte(c.Request.Method + " " + path + "\n" + string(data)))
	for receiptKey, saved := range s.receipts {
		if !now.Before(saved.expires) {
			delete(s.receipts, receiptKey)
		}
	}
	if key != "" {
		if saved, ok := s.receipts[key]; ok {
			if saved.hash != hash {
				return nil, "", conflict("同じキーで異なる要求は送信できません")
			}
			return saved.body, saved.etag, nil
		}
		if len(s.receipts) >= 10000 {
			return nil, "", &apiError{429, "CAPACITY", "再送記録が上限に達しました"}
		}
	}
	match := c.GetHeader("If-Match")
	if c.Request.Method == http.MethodPatch && match == "" {
		return nil, "", &apiError{428, "PRECONDITION_REQUIRED", "If-Matchが必要です"}
	}
	if match != "" && match != s.model.State.etag() {
		return nil, "", &apiError{412, "VERSION_CONFLICT", "別の操作が反映されました。最新の設定を確認してください"}
	}
	next := s.model
	var err error
	switch path {
	case "/timer":
		var payload settings
		if err = decode(data, &payload); err == nil {
			err = next.configure(payload)
		}
	case "/timer/commands":
		var payload command
		if err = decode(data, &payload); err == nil {
			if payload.Command == "adjust" && key == "" {
				err = &apiError{428, "PRECONDITION_REQUIRED", "加減算にはIdempotency-Keyが必要です"}
			} else {
				err = next.command(payload, now)
			}
		}
	case "/timer/blackout":
		var payload struct {
			Enabled *bool `json:"enabled"`
		}
		if err = decode(data, &payload); err == nil {
			if payload.Enabled == nil {
				err = invalid("enabledを指定してください")
			} else {
				next.Blackout = *payload.Enabled
			}
		}
	case "/timer/message":
		if c.Request.Method == http.MethodDelete {
			if len(bytes.TrimSpace(data)) > 0 {
				err = invalid("消去では本文を指定しないでください")
			} else {
				next.Message = Message{}
			}
		} else {
			var payload struct {
				Text    *string `json:"text"`
				Visible *bool   `json:"visible"`
			}
			if err = decode(data, &payload); err == nil {
				if payload.Text == nil && payload.Visible == nil {
					err = invalid("textまたはvisibleを指定してください")
				} else {
					if payload.Text != nil {
						next.Message = Message{*payload.Text, *payload.Text != ""}
					}
					if payload.Visible != nil {
						next.Message.Visible = *payload.Visible
					}
					if payload.Text != nil && *payload.Text == "" {
						next.Message.Visible = false
					}
					err = validateText(next.Message.Text)
					if err == nil && next.Message.Visible && next.Message.Text == "" {
						err = invalid("空のカンペは再表示できません")
					}
				}
			}
		}
	case "/timer/presets":
		var payload struct {
			Presets *[]string `json:"presets"`
		}
		if err = decode(data, &payload); err == nil {
			if payload.Presets == nil {
				err = invalid("presetsを指定してください")
			} else if err = validatePresets(*payload.Presets); err == nil {
				next.presets = append([]string{}, (*payload.Presets)...)
			}
		}
	case "/timer/bindings":
		var payload struct {
			Bindings *[]KeyBinding `json:"bindings"`
		}
		if err = decode(data, &payload); err == nil {
			if payload.Bindings == nil {
				err = invalid("bindingsを指定してください")
			} else if err = validateBindings(*payload.Bindings); err == nil {
				next.bindings = append([]KeyBinding{}, (*payload.Bindings)...)
			}
		}
	}
	if err != nil {
		return nil, "", err
	}
	changed := !reflect.DeepEqual(s.model, next)
	if changed && s.configPath != "" && (path == "/timer" || path == "/timer/presets" || path == "/timer/bindings") {
		if err := saveConfig(s.configPath, next); err != nil {
			log.Printf("save config: %v", err)
			return nil, "", &apiError{500, "CONFIG_SAVE_FAILED", "設定ファイルを保存できません"}
		}
	}
	if changed {
		next.Version++
	}
	state := next.snapshot(now)
	body, err := json.Marshal(state)
	if err != nil {
		return nil, "", err
	}
	if key != "" && len(body) > bodyLimit {
		return nil, "", invalid("応答が再送記録の上限を超えます")
	}
	s.model = next
	if key != "" {
		s.receipts[key] = receipt{hash, body, state.etag(), now.Add(10 * time.Minute)}
	}
	if changed {
		for client := range s.clients {
			select {
			case client <- struct{}{}:
			default:
			}
		}
	}
	return body, state.etag(), nil
}

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
