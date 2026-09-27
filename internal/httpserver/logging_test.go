package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"timeview/internal/systemlog"
)

func TestLoggingMiddlewareUsesSlogForRequestsAndPanics(t *testing.T) {
	original := slog.Default()
	t.Cleanup(func() { slog.SetDefault(original) })
	var output bytes.Buffer
	slog.SetDefault(systemlog.New(&output, slog.LevelInfo, true))

	router := gin.New()
	router.Use(AccessLogger(), Recovery())
	router.GET("/panic", func(*gin.Context) { panic("test panic") })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	got := output.String()
	for _, text := range []string{"ERR HTTP handler panic", "ERR HTTP request", "status=500"} {
		if !strings.Contains(got, text) {
			t.Fatalf("log does not contain %q: %s", text, got)
		}
	}
}
