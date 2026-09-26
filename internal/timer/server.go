package timer

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var configureGin sync.Once

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
	api.PATCH("/timer", s.mutation("/timer", true, updateSettings))
	api.POST("/timer/commands", s.mutation("/timer/commands", false, runCommand))
	api.PUT("/timer/blackout", s.mutation("/timer/blackout", false, updateBlackout))
	api.PUT("/timer/message", s.mutation("/timer/message", false, updateMessage))
	api.DELETE("/timer/message", s.mutation("/timer/message", false, updateMessage))
	api.PUT("/timer/presets", s.mutation("/timer/presets", true, updatePresets))
	api.PUT("/timer/bindings", s.mutation("/timer/bindings", true, updateBindings))
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
