package auditlog

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Entry struct {
	Time       time.Time `json:"time"`
	Type       string    `json:"type"`
	Action     string    `json:"action"`
	Result     string    `json:"result"`
	Method     string    `json:"method,omitempty"`
	Path       string    `json:"path,omitempty"`
	Client     string    `json:"client,omitempty"`
	StatusCode int       `json:"statusCode,omitempty"`
	ErrorCode  string    `json:"errorCode,omitempty"`
}

type Log struct {
	mu   sync.Mutex
	path string
	file *os.File
}

func Open(path string) (*Log, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &Log{path: path, file: file}, nil
}

func (l *Log) Write(entry Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	if err := json.NewEncoder(l.file).Encode(entry); err != nil {
		return err
	}
	return l.file.Sync()
}

func (l *Log) Read(limit int) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	file, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries := make([]Entry, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry Entry
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if len(entries) == limit {
			copy(entries, entries[1:])
			entries[len(entries)-1] = entry
		} else {
			entries = append(entries, entry)
		}
	}
	return entries, scanner.Err()
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
