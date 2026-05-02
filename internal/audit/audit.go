// Package audit emits one structured JSON record per privileged action
// (today: every reconcile/suspend/resume mutation; later: every denied
// request and every config change). Records go to an io.Writer — stdout
// in production so a log shipper (fluentd, vector, …) can route to a
// SIEM. Each line is independently parseable.
package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Outcome classifies what happened to the action.
type Outcome string

const (
	OutcomeOK     Outcome = "ok"
	OutcomeDenied Outcome = "denied"
	OutcomeError  Outcome = "error"
)

// Identity is the principal performing the action. Mirrors auth.Identity
// but without the import — keeps audit reusable from any package.
type Identity struct {
	Subject string   `json:"subject"`
	Email   string   `json:"email,omitempty"`
	Groups  []string `json:"groups,omitempty"`
}

// Resource identifies what the action targeted.
type Resource struct {
	Cluster string `json:"cluster"`
	Ns      string `json:"ns,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Name    string `json:"name,omitempty"`
}

// Record is one audit log line. Keep additive — downstream parsers must
// tolerate new fields appearing.
type Record struct {
	Timestamp  time.Time `json:"ts"`
	RequestID  string    `json:"request_id,omitempty"`
	Identity   Identity  `json:"identity"`
	Verb       string    `json:"verb"`
	Resource   Resource  `json:"resource"`
	Outcome    Outcome   `json:"outcome"`
	Error      string    `json:"error,omitempty"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
}

// Logger writes one Record per Record() call as a JSON line.
type Logger struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// New returns a Logger writing to w. Pass os.Stdout in production so the
// records ride alongside the application log on the same pod.
func New(w io.Writer) *Logger {
	return &Logger{enc: json.NewEncoder(w)}
}

// Discard returns a Logger whose Record is a no-op. Useful in tests where
// audit isn't the subject under test.
func Discard() *Logger { return New(io.Discard) }

// Record writes r as one JSON line, stamping Timestamp if zero.
func (l *Logger) Record(r Record) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now().UTC()
	}
	_ = l.enc.Encode(r)
}
