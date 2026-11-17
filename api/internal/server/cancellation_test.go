package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCancelQueuedJobCannotPublishResult(t *testing.T) {
	s, _, _ := setup(t)
	ctx, cancel := context.WithCancel(s.ctx)
	j := &job{state: JobState{ID: identifier(), Status: "queued", CreatedAt: time.Now()}, ctx: ctx, cancel: cancel}
	s.mu.Lock()
	s.jobs[j.state.ID] = j
	s.mu.Unlock()
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/jobs/"+j.state.ID+"/cancel", nil))
	if w.Code != 200 || ctx.Err() != context.Canceled {
		t.Fatalf("cancel: %d, %v", w.Code, ctx.Err())
	}
	s.run(j)
	state, _ := s.snapshot(j.state.ID)
	if state.Status != "cancelled" || state.DownloadURL != "" {
		t.Fatal(state)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/jobs/"+j.state.ID+"/download", nil))
	if w.Code != 409 {
		t.Fatal("cancelled result was downloadable")
	}
}
