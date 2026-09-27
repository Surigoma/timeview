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
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"timeview/internal/auditlog"
	"timeview/internal/httpserver"
	"timeview/web"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	listen := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	configPath := flag.String("config", "timeview-config.json", "JSON config file path")
	auditPath := flag.String("audit-log", "timeview-operations.jsonl", "JSON Lines operation log path")
	flag.Parse()
	absConfigPath, err := filepath.Abs(*configPath)
	if err != nil {
		return err
	}
	absAuditPath, err := filepath.Abs(*auditPath)
	if err != nil {
		return err
	}
	operations, err := auditlog.Open(absAuditPath)
	if err != nil {
		return fmt.Errorf("操作ログを開けません: %w", err)
	}
	defer func() {
		if value := recover(); value != nil {
			_ = operations.Write(auditlog.Entry{Type: "process", Action: "crash", Result: "failure"})
			_ = operations.Close()
			panic(value)
		}
		action, result := "stop", "success"
		if runErr != nil {
			action, result = "crash", "failure"
		}
		_ = operations.Write(auditlog.Entry{Type: "process", Action: action, Result: result})
		_ = operations.Close()
	}()
	if err := operations.Write(auditlog.Entry{Type: "process", Action: "start", Result: "success"}); err != nil {
		return fmt.Errorf("操作ログへ書き込めません: %w", err)
	}
	api, err := httpserver.NewWithConfig(absConfigPath)
	if err != nil {
		return err
	}
	api.SetAuditLog(operations)
	assets, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		return err
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return fmt.Errorf("frontend assets are missing; run task build: %w", err)
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
	for _, path := range []string{"/", "/control", "/touch", "/display", "/settings", "/logs"} {
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
		_ = server.Close()
	}()
	fmt.Printf("TimeView\nControl: http://%s/\nDisplay: http://%s/display\nConfig: %s\nAudit log: %s\nTimer state is in memory. Blackout is ON at startup.\n", *listen, *listen, absConfigPath, absAuditPath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
