package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

type JobUpdate struct {
	Type  string        `json:"type"`
	State *JobState     `json:"state,omitempty"`
	Entry *FileLogEntry `json:"entry,omitempty"`
}

func copyJobState(state JobState, history bool) JobState {
	state.Warnings = append([]string{}, state.Warnings...)
	if history {
		state.Log = append([]FileLogEntry(nil), state.Log...)
	} else {
		state.Log = nil
	}
	return state
}

// Called under s.mu. A slow reader reconnects to a fresh durable snapshot;
// it never blocks compression or silently loses an individual event.
func (s *Server) publish(j *job, event JobUpdate) {
	for subscriber := range j.subscribers {
		select {
		case subscriber <- event:
		default:
			close(subscriber)
			delete(j.subscribers, subscriber)
		}
	}
}

func (s *Server) publishJob(j *job) {
	if len(j.state.Log) > 0 {
		last := j.state.Log[len(j.state.Log)-1]
		if j.lastPublishedFile == nil || *j.lastPublishedFile != last {
			j.lastPublishedFile = &last
			s.publish(j, JobUpdate{Type: "file", Entry: &last})
		}
	}
	state := copyJobState(j.state, false)
	s.publish(j, JobUpdate{Type: "status", State: &state})
}

func (s *Server) jobWS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	_, exists := s.jobs[id]
	s.mu.Unlock()
	if !exists {
		fail(w, 404, fmt.Errorf("job not found"))
		return
	}
	// Handler has already checked Origin against the configured allowlist.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx := conn.CloseRead(r.Context())
	updates := make(chan JobUpdate, 128)
	s.mu.Lock()
	j := s.jobs[id]
	if j.subscribers == nil {
		j.subscribers = map[chan JobUpdate]struct{}{}
	}
	j.subscribers[updates] = struct{}{}
	initial := copyJobState(j.state, true)
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(j.subscribers, updates); s.mu.Unlock() }()
	write := func(event JobUpdate) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return conn.Write(writeCtx, websocket.MessageText, data)
	}
	if err = write(JobUpdate{Type: "snapshot", State: &initial}); err != nil {
		return
	}
	if terminal(initial.Status) {
		_ = conn.Close(websocket.StatusNormalClosure, "complete")
		return
	}
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case event, ok := <-updates:
			if !ok {
				_ = conn.Close(websocket.StatusTryAgainLater, "reconnect for snapshot")
				return
			}
			if err = write(event); err != nil {
				return
			}
			if event.Type == "status" && event.State != nil && terminal(event.State.Status) {
				_ = conn.Close(websocket.StatusNormalClosure, "complete")
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		case <-s.ctx.Done():
			_ = conn.Close(websocket.StatusGoingAway, "server stopping")
			return
		}
	}
}
