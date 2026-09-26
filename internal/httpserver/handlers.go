package httpserver

import (
	"crypto/sha256"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"

	"timeview/internal/timer"
)

const bodyLimit = 16 << 10

type receipt struct {
	hash    [32]byte
	body    []byte
	etag    string
	expires time.Time
}

type mutationFunc func(*timer.Model, *gin.Context, []byte, time.Time) error

func (s *Server) getTimer(c *gin.Context) {
	s.mu.Lock()
	state := s.model.Snapshot(s.now())
	s.mu.Unlock()
	c.Header("ETag", state.ETag())
	c.JSON(http.StatusOK, state)
}

func (s *Server) getPresets(c *gin.Context) {
	s.mu.Lock()
	presets := s.model.Presets()
	s.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"presets": presets})
}

func (s *Server) getBindings(c *gin.Context) {
	s.mu.Lock()
	bindings := s.model.Bindings()
	s.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"bindings": bindings})
}

func (s *Server) mutation(persist bool, apply mutationFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodDelete && c.ContentType() != "application/json" {
			writeError(c, &apiError{Status: 400, Code: "INVALID_CONTENT_TYPE", Message: "Content-Typeはapplication/jsonにしてください"})
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, bodyLimit))
		if err != nil {
			writeError(c, &apiError{Status: 413, Code: "BODY_TOO_LARGE", Message: "本文は16KiB以内にしてください"})
			return
		}
		body, etag, err := s.mutate(c, persist, apply, data)
		if err != nil {
			writeError(c, err)
			return
		}
		log.Printf("%s %s ok", c.Request.Method, c.Request.URL.Path)
		c.Header("ETag", etag)
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	}
}

func isBrowserMutation(c *gin.Context) bool {
	return c.GetHeader("X-Timeview-Client") == "browser" && c.GetHeader("Sec-Fetch-Site") == "same-origin"
}

func (s *Server) mutate(c *gin.Context, persist bool, apply mutationFunc, data []byte) ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.model.BrowserOnly && !isBrowserMutation(c) {
		return nil, "", &apiError{Status: 403, Code: "BROWSER_ONLY", Message: "現在はブラウザからの操作だけを受け付けます"}
	}
	now := s.now()
	if boot := c.GetHeader("X-Timeview-Instance"); boot != "" && boot != s.model.InstanceID {
		return nil, "", conflict("サーバーが再起動しました。状態を再取得してください")
	}
	key := c.GetHeader("Idempotency-Key")
	if len(key) > 128 {
		return nil, "", invalid("Idempotency-Keyは128文字以内です")
	}
	hash := sha256.Sum256([]byte(c.Request.Method + " " + c.Request.URL.Path + "\n" + string(data)))
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
			return nil, "", &apiError{Status: 429, Code: "CAPACITY", Message: "再送記録が上限に達しました"}
		}
	}
	match := c.GetHeader("If-Match")
	if c.Request.Method == http.MethodPatch && match == "" {
		return nil, "", &apiError{Status: 428, Code: "PRECONDITION_REQUIRED", Message: "If-Matchが必要です"}
	}
	if match != "" && match != s.model.State.ETag() {
		return nil, "", &apiError{Status: 412, Code: "VERSION_CONFLICT", Message: "別の操作が反映されました。最新の設定を確認してください"}
	}
	next := s.model
	if err := apply(&next, c, data, now); err != nil {
		return nil, "", err
	}
	changed := !reflect.DeepEqual(s.model, next)
	if changed && persist && s.configPath != "" {
		if err := timer.Save(s.configPath, next); err != nil {
			log.Printf("save config: %v", err)
			return nil, "", &apiError{Status: 500, Code: "CONFIG_SAVE_FAILED", Message: "設定ファイルを保存できません"}
		}
	}
	if changed {
		next.Version++
	}
	state := next.Snapshot(now)
	body, err := json.Marshal(state)
	if err != nil {
		return nil, "", err
	}
	if key != "" && len(body) > bodyLimit {
		return nil, "", invalid("応答が再送記録の上限を超えます")
	}
	s.model = next
	if key != "" {
		s.receipts[key] = receipt{hash, body, state.ETag(), now.Add(10 * time.Minute)}
	}
	if changed {
		for client := range s.clients {
			select {
			case client <- struct{}{}:
			default:
			}
		}
	}
	return body, state.ETag(), nil
}
