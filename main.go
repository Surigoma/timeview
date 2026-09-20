package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timeview/internal/timer"
	"timeview/web"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	configPath := flag.String("config", "timeview-config.json", "JSON config file path")
	flag.Parse()
	absConfigPath, err := filepath.Abs(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	api, err := timer.NewWithConfig(absConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	assets, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		log.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		log.Fatal("frontend assets are missing; run task build: ", err)
	}
	files := http.FileServer(http.FS(assets))
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	_ = router.SetTrustedProxies(nil)
	api.Register(router.Group("/api/v1"))
	serveIndex := func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
	for _, path := range []string{"/", "/control", "/display", "/settings"} {
		router.Match([]string{http.MethodGet, http.MethodHead}, path, serveIndex)
	}
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "APIが見つかりません"}})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusMethodNotAllowed)
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(c.Writer, c.Request)
	})
	server := &http.Server{Addr: *listen, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
		_ = server.Close()
	}()
	fmt.Printf("TimeView\nControl: http://%s/\nDisplay: http://%s/display\nConfig: %s\nTimer state is in memory. Blackout is ON at startup.\n", *listen, *listen, absConfigPath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
