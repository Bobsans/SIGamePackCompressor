package server

import (
	"context"
	"io"
	"time"

	"sigamepackcompressor/internal/siq"
)

type FileLogEntry struct {
	Sequence  int       `json:"sequence"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	OldSize   int64     `json:"old_size"`
	NewSize   *int64    `json:"new_size,omitempty"`
	Warning   string    `json:"warning,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// Keep the active file and complete history in the job snapshot.
// Existing WebSocket messages remain unchanged for older clients.
func (s *Server) withFileLog(j *job, transform siq.Transform) siq.Transform {
	return func(ctx context.Context, m siq.Media, r io.Reader) (string, string, error) {
		s.mu.Lock()
		if !terminal(j.state.Status) && ctx.Err() == nil {
			j.state.Log = appendFileLog(j.state.Log, FileLogEntry{
				Sequence: j.state.Progress + 1, Name: m.Name, Type: m.Type,
				Status: "processing", OldSize: m.Size, StartedAt: time.Now().UTC(),
			})
			s.persist(j)
		}
		s.mu.Unlock()
		return transform(ctx, m, r)
	}
}

func appendFileLog(log []FileLogEntry, entry FileLogEntry) []FileLogEntry {
	return append(log, entry)
}

// Called under s.mu after the archive has decided whether to keep the result.
func completeFileLog(state *JobState, m siq.Media, size int64, warning string) {
	entry := FileLogEntry{Sequence: state.Progress, Name: m.Name, Type: m.Type, OldSize: m.Size, NewSize: &size, Warning: warning, StartedAt: time.Now().UTC(), Status: "unchanged"}
	if size < m.Size {
		entry.Status = "compressed"
	}
	last := len(state.Log) - 1
	if last >= 0 && state.Log[last].Status == "processing" && state.Log[last].Sequence == entry.Sequence && state.Log[last].Name == m.Name {
		entry.StartedAt = state.Log[last].StartedAt
		state.Log[last] = entry
	} else {
		// Copied HTML resources do not pass through the media transformer.
		state.Log = appendFileLog(state.Log, entry)
	}
}

func interruptFileLog(state *JobState) {
	last := len(state.Log) - 1
	if last >= 0 && state.Log[last].Status == "processing" {
		state.Log[last].Status = "interrupted"
	}
}
