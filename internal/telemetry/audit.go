package telemetry

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type auditEntry struct {
	Ts     string            `json:"ts"`
	Event  string            `json:"event"`
	User   string            `json:"user"`
	Actor  string            `json:"actor,omitempty"`
	Detail map[string]string `json:"detail,omitempty"`
}

type AuditLog struct {
	f  *os.File
	mu sync.Mutex
}

func NewAuditLog(path string) (*AuditLog, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &AuditLog{f: f}, nil
}

func (a *AuditLog) Log(event, user, actor string, detail map[string]string) {
	entry := auditEntry{
		Ts:     time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Event:  event,
		User:   user,
		Actor:  actor,
		Detail: detail,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')
	a.mu.Lock()
	defer a.mu.Unlock()
	a.f.Write(data)
}

func (a *AuditLog) Close() error {
	return a.f.Close()
}
