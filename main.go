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
	"time"

	"timeview/internal/timer"
	"timeview/web"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	flag.Parse()
	assets, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		log.Fatal(err)
	}
	files := http.FileServer(http.FS(assets))
	mux := http.NewServeMux()
	mux.Handle("/api/", timer.New())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		switch r.URL.Path {
		case "/", "/control", "/display", "/settings":
			data, err := fs.ReadFile(assets, "index.html")
			if err != nil {
				http.Error(w, "Run task build first", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(data)
		default:
			files.ServeHTTP(w, r)
		}
	})
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
		_ = server.Close()
	}()
	fmt.Printf("TimeView\nControl: http://%s/\nDisplay: http://%s/display\nState is in memory. Blackout is ON at startup.\n", *listen, *listen)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
