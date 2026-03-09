package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type State string

const (
	StateRunning   State = "running"
	StateCompleted State = "completed"
)

type DaemonStatus struct {
	State        State     `json:"state"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	FilesScanned int       `json:"files_scanned"`
	ExpiredFiles int       `json:"expired_files"`

	mu   sync.Mutex `json:"-"`
	path string     `json:"-"`
}

func statusPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".dinha", "daemon_status.json")
}

// NewWriter creates a status writer that persists to disk on every update.
func NewWriter() *DaemonStatus {
	return &DaemonStatus{path: statusPath()}
}

func (s *DaemonStatus) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = StateRunning
	s.StartedAt = time.Now()
	s.FilesScanned = 0
	s.ExpiredFiles = 0
	s.FinishedAt = time.Time{}
	s.save()
}

func (s *DaemonStatus) IncrFiles(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FilesScanned += n
	s.save()
}

func (s *DaemonStatus) Finish(expiredFiles int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = StateCompleted
	s.FinishedAt = time.Now()
	s.ExpiredFiles = expiredFiles
	s.save()
}

func (s *DaemonStatus) save() {
	if s.path == "" {
		return
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	os.WriteFile(s.path, data, 0644)
}

// Read loads the daemon status from disk. Returns nil if not available.
func Read() *DaemonStatus {
	p := statusPath()
	if p == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var s DaemonStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}
