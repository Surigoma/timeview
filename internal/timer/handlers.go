package timer

import (
	"crypto/sha256"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
)

const bodyLimit = 16 << 10

type receipt struct {
	hash    [32]byte
	body    []byte
	etag    string
	expires time.Time
}

type mutationFunc func(*model, *gin.Context, []byte, time.Time) error

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

func (s *Server) mutation(path string, persist bool, apply mutationFunc) gin.HandlerFunc {
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
		body, etag, err := s.mutate(c, path, persist, apply, data)
		if err != nil {
			writeError(c, err)
			return
		}
		log.Printf("%s %s ok", c.Request.Method, path)
		c.Header("ETag", etag)
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	}
}

func isBrowserMutation(c *gin.Context) bool {
	return c.GetHeader("X-Timeview-Client") == "browser" && c.GetHeader("Sec-Fetch-Site") == "same-origin"
}

func (s *Server) mutate(c *gin.Context, path string, persist bool, apply mutationFunc, data []byte) ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.model.BrowserOnly && !isBrowserMutation(c) {
		return nil, "", &apiError{403, "BROWSER_ONLY", "現在はブラウザからの操作だけを受け付けます"}
	}
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
	if err := apply(&next, c, data, now); err != nil {
		return nil, "", err
	}
	changed := !reflect.DeepEqual(s.model, next)
	if changed && persist && s.configPath != "" {
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
