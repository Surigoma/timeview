package timer

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const bodyLimit = 16 << 10

type receipt struct {
	hash    [32]byte
	body    []byte
	etag    string
	expires time.Time
}
type bucket struct {
	write, read float64
	last        time.Time
}

type Server struct {
	mu       sync.Mutex
	model    model
	receipts map[string]receipt
	clients  map[chan struct{}]struct{}
	limits   map[string]*bucket
	now      func() time.Time
}

func New() *Server {
	return &Server{model: newModel(time.Now()), receipts: map[string]receipt{}, clients: map[chan struct{}]struct{}{}, limits: map[string]*bucket{}, now: time.Now}
}

func writeError(w http.ResponseWriter, err error) {
	var e *apiError
	if !errors.As(err, &e) {
		e = &apiError{500, "INTERNAL_ERROR", "内部エラーが発生しました"}
	}
	if e.status == 429 {
		w.Header().Set("Retry-After", "1")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": e.code, "message": e.message}})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func decode(data []byte, v any) error {
	if !utf8.Valid(data) {
		return invalid("UTF-8で送信してください")
	}
	// Reject null fields as well as unknown fields: pointer omission must not hide invalid input.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return &apiError{400, "INVALID_JSON", "JSONオブジェクトを指定してください"}
	}
	for _, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return invalid("nullは指定できません")
		}
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return invalid("フィールド名または値の型が不正です")
	}
	return nil
}

func (s *Server) allow(r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if host == "" {
		host = r.RemoteAddr
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, b := range s.limits {
		if now.Sub(b.last) > time.Minute {
			delete(s.limits, key)
		}
	}
	b := s.limits[host]
	if b == nil {
		if len(s.limits) >= 1024 {
			return false
		}
		b = &bucket{20, 30, now}
		s.limits[host] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.write = min(20, b.write+elapsed*10)
	b.read = min(30, b.read+elapsed*30)
	b.last = now
	tokens := &b.write
	if r.Method == "GET" {
		tokens = &b.read
	}
	if *tokens < 1 {
		return false
	}
	*tokens--
	return true
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !s.allow(r) {
		writeError(w, &apiError{429, "RATE_LIMITED", "操作間隔を空けて再試行してください"})
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/api/v1")
	if p == "/health" && r.Method == "GET" {
		writeJSON(w, map[string]string{"status": "ok"})
		return
	}
	if p == "/timer/events" && r.Method == "GET" {
		s.events(w, r)
		return
	}
	if r.Method == "GET" {
		s.mu.Lock()
		state := s.model.snapshot(s.now())
		presets := append([]string{}, s.model.presets...)
		s.mu.Unlock()
		switch p {
		case "/timer":
			w.Header().Set("ETag", state.etag())
			writeJSON(w, state)
		case "/timer/presets":
			writeJSON(w, map[string]any{"presets": presets})
		default:
			writeError(w, &apiError{404, "NOT_FOUND", "APIが見つかりません"})
		}
		return
	}
	switch r.Method + " " + p {
	case "PATCH /timer", "POST /timer/commands", "PUT /timer/blackout", "PUT /timer/message", "DELETE /timer/message", "PUT /timer/presets":
	default:
		writeError(w, &apiError{404, "NOT_FOUND", "APIが見つかりません"})
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, bodyLimit))
	if err != nil {
		writeError(w, &apiError{413, "BODY_TOO_LARGE", "本文は16KiB以内にしてください"})
		return
	}
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Method != "DELETE" && mediaType != "application/json" {
		writeError(w, &apiError{400, "INVALID_CONTENT_TYPE", "Content-Typeはapplication/jsonにしてください"})
		return
	}
	body, etag, err := s.mutate(r, p, data)
	if err != nil {
		writeError(w, err)
		return
	}
	log.Printf("%s %s ok", r.Method, p)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("ETag", etag)
	_, _ = w.Write(body)
}

func (s *Server) mutate(r *http.Request, path string, data []byte) ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if boot := r.Header.Get("X-Timeview-Instance"); boot != "" && boot != s.model.InstanceID {
		return nil, "", conflict("サーバーが再起動しました。状態を再取得してください")
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) > 128 {
		return nil, "", invalid("Idempotency-Keyは128文字以内です")
	}
	hash := sha256.Sum256([]byte(r.Method + " " + path + "\n" + string(data)))
	for k, rec := range s.receipts {
		if !now.Before(rec.expires) {
			delete(s.receipts, k)
		}
	}
	if key != "" {
		if rec, ok := s.receipts[key]; ok {
			if rec.hash != hash {
				return nil, "", conflict("同じキーで異なる要求は送信できません")
			}
			return rec.body, rec.etag, nil
		}
		if len(s.receipts) >= 10000 {
			return nil, "", &apiError{429, "CAPACITY", "再送記録が上限に達しました"}
		}
	}
	match := r.Header.Get("If-Match")
	if r.Method == "PATCH" && match == "" {
		return nil, "", &apiError{428, "PRECONDITION_REQUIRED", "If-Matchが必要です"}
	}
	if match != "" && match != s.model.State.etag() {
		return nil, "", &apiError{412, "VERSION_CONFLICT", "別の操作が反映されました。最新の設定を確認してください"}
	}
	next := s.model
	var err error
	switch path {
	case "/timer":
		var p settings
		if err = decode(data, &p); err == nil {
			err = next.configure(p)
		}
	case "/timer/commands":
		var c command
		if err = decode(data, &c); err == nil {
			if c.Command == "adjust" && key == "" {
				err = &apiError{428, "PRECONDITION_REQUIRED", "加減算にはIdempotency-Keyが必要です"}
			} else {
				err = next.command(c, now)
			}
		}
	case "/timer/blackout":
		var p struct {
			Enabled *bool `json:"enabled"`
		}
		if err = decode(data, &p); err == nil {
			if p.Enabled == nil {
				err = invalid("enabledを指定してください")
			} else {
				next.Blackout = *p.Enabled
			}
		}
	case "/timer/message":
		if r.Method == "DELETE" {
			if len(bytes.TrimSpace(data)) > 0 {
				err = invalid("消去では本文を指定しないでください")
			} else {
				next.Message = Message{}
			}
		} else {
			var p struct {
				Text    *string `json:"text"`
				Visible *bool   `json:"visible"`
			}
			if err = decode(data, &p); err == nil {
				if p.Text == nil && p.Visible == nil {
					err = invalid("textまたはvisibleを指定してください")
				} else {
					if p.Text != nil {
						next.Message = Message{*p.Text, *p.Text != ""}
					}
					if p.Visible != nil {
						next.Message.Visible = *p.Visible
					}
					if p.Text != nil && *p.Text == "" {
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
		var p struct {
			Presets *[]string `json:"presets"`
		}
		if err = decode(data, &p); err == nil {
			if p.Presets == nil || len(*p.Presets) > 9 {
				err = invalid("定型文は最大9件です")
			} else {
				for _, text := range *p.Presets {
					if err = validateText(text); err != nil {
						break
					}
				}
				if err == nil {
					next.presets = append([]string{}, (*p.Presets)...)
				}
			}
		}
	}
	if err != nil {
		return nil, "", err
	}
	changed := !reflect.DeepEqual(s.model, next)
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
		for ch := range s.clients {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
	return body, state.etag(), nil
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if _, ok := w.(http.Flusher); !ok {
		writeError(w, errors.New("streaming unsupported"))
		return
	}
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	if len(s.clients) >= 50 {
		s.mu.Unlock()
		writeError(w, &apiError{429, "CAPACITY", "表示接続が上限に達しました"})
		return
	}
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.clients, ch); s.mu.Unlock() }()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	controller := http.NewResponseController(w)
	for {
		s.mu.Lock()
		state := s.model.snapshot(s.now())
		s.mu.Unlock()
		data, _ := json.Marshal(state)
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprintf(w, "event: state\ndata: %s\n\n", data); err != nil {
			return
		}
		if err := controller.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		case <-ch:
		}
	}
}
