package server

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigamepackcompressor/internal/config"
	"sigamepackcompressor/internal/media"
	"sigamepackcompressor/internal/siq"
)

func TestFileLogShowsActiveFileAndCopiesSnapshots(t *testing.T) {
	s, _, _ := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	j := &job{state: JobState{ID: identifier(), Status: "running"}, ctx: ctx, cancel: cancel}
	s.mu.Lock()
	s.jobs[j.state.ID] = j
	s.mu.Unlock()
	m := siq.Media{Name: "Images/question.png", Type: "image", Size: 4000}
	transform := s.withFileLog(j, func(context.Context, siq.Media, io.Reader) (string, string, error) {
		state, ok := s.snapshot(j.state.ID)
		if !ok || len(state.Log) != 1 || state.Log[0].Status != "processing" || state.Log[0].Name != m.Name || state.Log[0].OldSize != m.Size || state.Log[0].NewSize != nil {
			t.Fatalf("active file not exposed before conversion: %+v", state.Log)
		}
		return "", "", nil
	})
	if _, _, err := transform(ctx, m, strings.NewReader("image")); err != nil {
		t.Fatal(err)
	}
	before, _ := s.snapshot(j.state.ID)
	s.mu.Lock()
	j.state.Progress++
	completeFileLog(&j.state, m, 1000, "")
	s.persist(j)
	s.mu.Unlock()
	after, _ := s.snapshot(j.state.ID)
	if len(after.Log) != 1 || after.Log[0].Status != "compressed" || *after.Log[0].NewSize != 1000 {
		t.Fatal(after.Log)
	}
	if before.Log[0].Status != "processing" || before.Log[0].NewSize != nil {
		t.Fatal("snapshot shares mutable history with the running job")
	}
	if after.Log[0].Sequence != before.Log[0].Sequence || !after.Log[0].StartedAt.Equal(before.Log[0].StartedAt) {
		t.Fatal("active log identity changed on completion")
	}
}

func TestJobFileLogIsCompleteAndSurvivesRestart(t *testing.T) {
	c := config.Defaults()
	c.StoragePath = t.TempDir()
	c.WebPath = ""
	s, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	id := identifier()
	path := filepath.Join(c.StoragePath, "uploads", id+".siq")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	var xml strings.Builder
	xml.WriteString(`<package version="5"><rounds><round><themes><theme><questions><question><params><param name="question" type="content">`)
	for i := 0; i < 25; i++ {
		name := fmt.Sprintf("%02d.png", i)
		fmt.Fprintf(&xml, `<item type="image" isRef="true">%s</item>`, name)
		w, _ := z.Create("Images/" + name)
		io.WriteString(w, "retained resource")
	}
	xml.WriteString(`</param></params></question></questions></theme></themes></round></rounds></package>`)
	w, _ := z.Create("content.xml")
	io.WriteString(w, xml.String())
	z.Close()
	f.Close()
	p, err := siq.Open(path, c.Limits)
	if err != nil {
		t.Fatal(err)
	}
	u := Upload{ID: id, Filename: "test.siq", Analysis: p.Info}
	p.Close()
	s.mu.Lock()
	s.uploads[id] = u
	err = s.store.save("go_uploads", id, u)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	o := media.Defaults()
	o.Images.Enabled = false
	state, err := s.submit(id, o)
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, state.ID)
	if finished.Status != "completed" || len(finished.Log) != 25 {
		t.Fatalf("completed log: %+v", finished)
	}
	if finished.Log[0].Sequence != 1 || finished.Log[24].Sequence != 25 {
		t.Fatal("wrong retained range")
	}
	for _, entry := range finished.Log {
		if entry.Status != "unchanged" || entry.NewSize == nil || *entry.NewSize != entry.OldSize {
			t.Fatal(entry)
		}
	}
	s.Close()
	s, err = New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	recorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/api/jobs/"+state.ID, nil))
	var restored JobState
	if err = json.Unmarshal(recorder.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != 200 || len(restored.Log) != 25 || restored.Log[24].Name != finished.Log[24].Name {
		t.Fatalf("log lost after restart: %s", recorder.Body.String())
	}
}

func TestInterruptedAndUnchangedFileLogs(t *testing.T) {
	s, _, c := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{state: JobState{ID: identifier(), Status: "running", Log: []FileLogEntry{{Sequence: 1, Name: "Video/clip.mp4", Status: "processing"}}}, ctx: ctx, cancel: cancel}
	s.mu.Lock()
	s.jobs[j.state.ID] = j
	s.mu.Unlock()
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/jobs/"+j.state.ID+"/cancel", nil))
	state, _ := s.snapshot(j.state.ID)
	if state.Log[0].Status != "interrupted" {
		t.Fatal("cancel left a processing row")
	}
	restarted := JobState{ID: identifier(), Status: "running", Log: []FileLogEntry{{Sequence: 1, Name: "Video/clip.mp4", Status: "processing"}}}
	if err := s.store.saveJob(restarted); err != nil {
		t.Fatal(err)
	}
	// Read a separate database instance only after the prior worker is idle.
	other, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	state, _ = other.snapshot(restarted.ID)
	if state.Status != "failed" || state.Log[0].Status != "interrupted" {
		t.Fatal(state)
	}
	state = JobState{Progress: 1}
	m := siq.Media{Name: "Images/broken.png", Type: "image", Size: 200}
	completeFileLog(&state, m, 200, "decoder failed")
	if state.Log[0].Status != "unchanged" || state.Log[0].Warning != "decoder failed" {
		t.Fatal(state.Log)
	}
}
