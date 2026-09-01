package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"gopkg.in/lumberjack.v2"
)

type auditEntry struct {
	Ts     string            `json:"ts"`
	Event  string            `json:"event"`
	User   string            `json:"user"`
	Actor  string            `json:"actor,omitempty"`
	Detail map[string]string `json:"detail,omitempty"`
}

type AuditLog struct {
	w  io.WriteCloser
	mu sync.Mutex
}

func NewAuditLog(path string, maxSizeMB, maxBackups int) *AuditLog {
	return &AuditLog{
		w: &lumberjack.Logger{
			Filename:   path,
			MaxSize:    maxSizeMB,
			MaxBackups: maxBackups,
		},
	}
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
	a.w.Write(data)
}

func (a *AuditLog) Close() error {
	return a.w.Close()
}
