package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

type remote struct {
	base   string
	client *http.Client
}

type state struct {
	InstanceID string `json:"instanceId"`
}

type midiBinding struct {
	Status uint8 `json:"status"`
	Data1  uint8 `json:"data1"`
}

type keyBinding struct {
	Action string       `json:"action"`
	Code   string       `json:"code"`
	Ctrl   bool         `json:"ctrl"`
	Shift  bool         `json:"shift"`
	Alt    bool         `json:"alt"`
	Meta   bool         `json:"meta"`
	MIDI   *midiBinding `json:"midi,omitempty"`
}

type inputEvent struct {
	Code                   string
	Ctrl, Shift, Alt, Meta bool
	MIDI                   *midiBinding
}

var startInputs = startPlatformInputs

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("timeview-remote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	server := flags.String("server", "http://127.0.0.1:8080", "TimeView server URL")
	timeout := flags.Duration("timeout", 5*time.Second, "request timeout")
	if err := flags.Parse(args); err != nil {
		return errors.New("引数が不正です")
	}
	parsed, err := url.Parse(*server)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("サーバーURLが不正です")
	}
	command := flags.Args()
	if len(command) == 0 {
		return errors.New("操作を指定してください: listen, status, start, pause, reset, add, subtract, blackout, reveal, hide, show, clear, message, preset")
	}
	r := remote{strings.TrimRight(*server, "/") + "/api/v1/timer", &http.Client{Timeout: *timeout}}
	if command[0] == "listen" {
		if len(command) != 1 {
			return errors.New("listenに値は指定できません")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return r.listen(ctx, output)
	}
	if command[0] == "status" {
		if len(command) != 1 {
			return errors.New("statusに値は指定できません")
		}
		body, _, err := r.get("")
		if err == nil {
			_, err = fmt.Fprintln(output, string(body))
		}
		return err
	}
	if err := r.execute(command); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "OK %s\n", command[0])
	return err
}

func (r remote) execute(command []string) error {
	path, method, body, err := requestFor(command, r)
	if err != nil {
		return err
	}
	_, current, err := r.get("")
	if err != nil {
		return err
	}
	return r.send(path, method, body, current.InstanceID)
}

func (r remote) listen(ctx context.Context, output io.Writer) error {
	data, _, err := r.get("/bindings")
	if err != nil {
		return err
	}
	var response struct {
		Bindings []keyBinding `json:"bindings"`
	}
	if json.Unmarshal(data, &response) != nil || len(response.Bindings) == 0 {
		return errors.New("TimeViewのキー割り当てが不正です")
	}
	events := make(chan inputEvent, 32)
	stop, midiDevices, err := startInputs(events)
	if err != nil {
		return err
	}
	defer stop()
	if _, err := fmt.Fprintf(output, "入力監視を開始しました（MIDI入力: %d台、終了: Ctrl+C）\n", midiDevices); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-events:
			action := actionFor(response.Bindings, event)
			command := commandForAction(action)
			if command == nil {
				continue
			}
			if err := r.execute(command); err != nil {
				_, _ = fmt.Fprintf(output, "ERROR %s: %v\n", action, err)
				continue
			}
			_, _ = fmt.Fprintf(output, "OK %s\n", action)
		}
	}
}

func actionFor(bindings []keyBinding, event inputEvent) string {
	for _, binding := range bindings {
		if event.MIDI != nil && binding.MIDI != nil && *event.MIDI == *binding.MIDI {
			return binding.Action
		}
		if event.MIDI == nil && binding.Code == event.Code && binding.Ctrl == event.Ctrl && binding.Shift == event.Shift && binding.Alt == event.Alt && binding.Meta == event.Meta {
			return binding.Action
		}
	}
	return ""
}

func commandForAction(action string) []string {
	if strings.HasPrefix(action, "preset") {
		number, err := strconv.Atoi(strings.TrimPrefix(action, "preset"))
		if err == nil && number >= 0 && number < 9 {
			return []string{"preset", strconv.Itoa(number + 1)}
		}
		return nil
	}
	switch action {
	case "start", "pause", "reset", "add", "subtract", "blackout", "reveal", "hide", "show", "clear":
		return []string{action}
	default:
		return nil
	}
}

func requestFor(args []string, r remote) (string, string, any, error) {
	switch args[0] {
	case "start", "pause", "reset":
		if len(args) != 1 {
			return "", "", nil, errors.New("この操作に値は指定できません")
		}
		return "/commands", http.MethodPost, map[string]any{"command": args[0]}, nil
	case "add", "subtract":
		delta := int64(60)
		if len(args) > 2 {
			return "", "", nil, errors.New("秒数は1つだけ指定してください")
		}
		if len(args) == 2 {
			value, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil || value < 1 || value > 3600 {
				return "", "", nil, errors.New("秒数は1～3600で指定してください")
			}
			delta = value
		}
		if args[0] == "subtract" {
			delta = -delta
		}
		return "/commands", http.MethodPost, map[string]any{"command": "adjust", "deltaSeconds": delta}, nil
	case "blackout", "reveal":
		if len(args) != 1 {
			return "", "", nil, errors.New("この操作に値は指定できません")
		}
		return "/blackout", http.MethodPut, map[string]any{"enabled": args[0] == "blackout"}, nil
	case "hide", "show":
		if len(args) != 1 {
			return "", "", nil, errors.New("この操作に値は指定できません")
		}
		return "/message", http.MethodPut, map[string]any{"visible": args[0] == "show"}, nil
	case "clear":
		if len(args) != 1 {
			return "", "", nil, errors.New("この操作に値は指定できません")
		}
		return "/message", http.MethodDelete, nil, nil
	case "message":
		if len(args) < 2 {
			return "", "", nil, errors.New("送信するメッセージを指定してください")
		}
		return "/message", http.MethodPut, map[string]any{"text": strings.Join(args[1:], " ")}, nil
	case "preset":
		if len(args) != 2 {
			return "", "", nil, errors.New("定型文番号を1～9で指定してください")
		}
		number, err := strconv.Atoi(args[1])
		if err != nil || number < 1 || number > 9 {
			return "", "", nil, errors.New("定型文番号を1～9で指定してください")
		}
		data, _, err := r.get("/presets")
		if err != nil {
			return "", "", nil, err
		}
		var presets struct {
			Presets []string `json:"presets"`
		}
		if json.Unmarshal(data, &presets) != nil || number > len(presets.Presets) || presets.Presets[number-1] == "" {
			return "", "", nil, errors.New("指定した定型文は未設定です")
		}
		return "/message", http.MethodPut, map[string]any{"text": presets.Presets[number-1]}, nil
	default:
		return "", "", nil, fmt.Errorf("未対応の操作です: %s", args[0])
	}
}

func (r remote) get(path string) ([]byte, state, error) {
	response, err := r.client.Get(r.base + path)
	if err != nil {
		return nil, state{}, fmt.Errorf("TimeViewへ接続できません: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, state{}, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, state{}, responseError(response.StatusCode, body)
	}
	var current state
	if path == "" && (json.Unmarshal(body, &current) != nil || current.InstanceID == "") {
		return nil, state{}, errors.New("TimeViewの応答が不正です")
	}
	return body, current, nil
}

func (r remote) send(path, method string, value any, instanceID string) error {
	var body io.Reader
	if value != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, r.base+path, body)
	if err != nil {
		return err
	}
	if value != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	request.Header.Set("Idempotency-Key", hex.EncodeToString(id))
	request.Header.Set("X-Timeview-Instance", instanceID)
	response, err := r.client.Do(request)
	if err != nil {
		return fmt.Errorf("TimeViewへ接続できません: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return responseError(response.StatusCode, data)
	}
	return nil
}

func responseError(status int, data []byte) error {
	var response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &response) == nil && response.Error.Message != "" {
		return errors.New(response.Error.Message)
	}
	return fmt.Errorf("TimeViewがHTTP %dを返しました", status)
}
