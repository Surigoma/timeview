package httpserver

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"timeview/internal/auditlog"
	"timeview/internal/systemlog"
	"timeview/internal/timer"
)

var configureGin sync.Once

type Server struct {
	mu         sync.Mutex
	model      timer.Model
	configPath string
	receipts   map[string]receipt
	clients    map[chan struct{}]struct{}
	limits     map[string]*clientLimit
	now        func() time.Time
	handler    http.Handler
	handlerOne sync.Once
	audit      *auditlog.Log
	logLevel   *slog.LevelVar
}

func (s *Server) SetAuditLog(log *auditlog.Log) { s.audit = log }

func (s *Server) SetLogLevel(level *slog.LevelVar) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logLevel = level
	_ = systemlog.SetLevel(level, s.model.LogLevel)
}

func prepareGin() {
	configureGin.Do(func() {
		gin.SetMode(gin.ReleaseMode)
		gin.EnableJsonDecoderDisallowUnknownFields()
	})
}

func New() *Server {
	prepareGin()
	return &Server{model: timer.New(time.Now()), receipts: map[string]receipt{}, clients: map[chan struct{}]struct{}{}, limits: map[string]*clientLimit{}, now: time.Now}
}

func NewWithConfig(path string) (*Server, error) {
	prepareGin()
	loaded, err := timer.Load(path, time.Now())
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
	api.GET("/logs", s.getLogs)
	api.PATCH("/timer", s.mutation(true, updateSettings))
	api.POST("/timer/commands", s.mutation(false, runCommand))
	api.PUT("/timer/blackout", s.mutation(false, updateBlackout))
	api.PUT("/timer/message", s.mutation(false, updateMessage))
	api.DELETE("/timer/message", s.mutation(false, updateMessage))
	api.PUT("/timer/presets", s.mutation(true, updatePresets))
	api.PUT("/timer/bindings", s.mutation(true, updateBindings))
}

// ServeHTTP keeps Server usable with httptest and as a standalone net/http handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handlerOne.Do(func() {
		router := gin.New()
		router.Use(Recovery())
		_ = router.SetTrustedProxies(nil)
		s.Register(router.Group("/api/v1"))
		router.NoRoute(func(c *gin.Context) {
			writeError(c, &apiError{Status: 404, Code: "NOT_FOUND", Message: "APIが見つかりません"})
		})
		s.handler = router
	})
	s.handler.ServeHTTP(w, r)
}
