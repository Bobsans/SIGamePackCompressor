package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sigamepackcompressor/internal/config"
)

func setup(t *testing.T) (*Server, *httptest.Server, config.Config) {
	t.Helper()
	c := config.Defaults()
	c.StoragePath = t.TempDir()
	c.WebPath = ""
	s, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	return s, ts, c
}
func multipartPack(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	w, _ := z.Create("content.xml")
	io.WriteString(w, `<package version="3" name="Test"><rounds><round><themes><theme><questions><question price="100"><scenario><atom>Question</atom><atom type="marker"/><atom>Answer</atom></scenario><right><answer>Answer</answer></right></question></questions></theme></themes></round></rounds></package>`)
	z.Close()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "test.siq")
	part.Write(archive.Bytes())
	mw.Close()
	return &body, mw.FormDataContentType()
}
func uploadPack(t *testing.T, base string) Upload {
	t.Helper()
	body, ct := multipartPack(t)
	resp, err := http.Post(base+"/api/uploads", ct, body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload %d: %s", resp.StatusCode, b)
	}
	var u Upload
	json.NewDecoder(resp.Body).Decode(&u)
	return u
}
func waitJob(t *testing.T, s *Server, id string) JobState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, _ := s.snapshot(id)
		if terminal(state.Status) {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return JobState{}
}
func TestUploadAnalyzeConfigureCompressDownload(t *testing.T) {
	s, ts, _ := setup(t)
	u := uploadPack(t, ts.URL)
	if u.Analysis.Version != "3" || u.Analysis.Questions != 1 {
		t.Fatal(u)
	}
	resp, err := http.Post(ts.URL+"/api/uploads/"+u.ID+"/jobs", "application/json", strings.NewReader(`{"output_version":"5","images":{"quality":70}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var job JobState
	json.NewDecoder(resp.Body).Decode(&job)
	if resp.StatusCode != 202 {
		t.Fatal(resp.StatusCode, job)
	}
	if job.Options.Images.MaxWidth == 0 || job.Options.Images.Quality != 70 {
		t.Fatal("partial defaults lost")
	}
	state := waitJob(t, s, job.ID)
	if state.Status != "completed" {
		t.Fatal(state)
	}
	result, err := http.Get(ts.URL + state.DownloadURL)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()
	b, _ := io.ReadAll(result.Body)
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	f, _ := z.Open("content.xml")
	xml, _ := io.ReadAll(f)
	f.Close()
	if !bytes.Contains(xml, []byte(`version="5"`)) || !bytes.Contains(xml, []byte(`name="answer"`)) {
		t.Fatal(string(xml))
	}
	// The original remains reusable with different options.
	resp2, err := http.Post(ts.URL+"/api/uploads/"+u.ID+"/jobs", "application/json", strings.NewReader(`{"output_version":"source"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var second JobState
	json.NewDecoder(resp2.Body).Decode(&second)
	if second.ID == job.ID {
		t.Fatal("job overwritten")
	}
	if waitJob(t, s, second.ID).Status != "completed" {
		t.Fatal("second run failed")
	}
}
func TestValidationAndOrigins(t *testing.T) {
	_, ts, _ := setup(t)
	u := uploadPack(t, ts.URL)
	for _, body := range []string{`{"video":{"preset":";bad"}}`, `{"output_version":"4"}`, `{"unknown":true}`, `{} {}`} {
		r, err := http.Post(ts.URL+"/api/uploads/"+u.ID+"/jobs", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode < 400 {
			t.Fatalf("accepted %s", body)
		}
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/options", nil)
	req.Header.Set("Origin", "https://untrusted.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatal("unexpected origin accepted")
	}
}
func TestRestartRetainsUploads(t *testing.T) {
	c := config.Defaults()
	c.StoragePath = t.TempDir()
	s, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	u := uploadPack(t, ts.URL)
	s.mu.Lock()
	state := JobState{ID: identifier(), UploadID: u.ID, Status: "running"}
	s.store.save("go_jobs", state.ID, state)
	s.mu.Unlock()
	ts.Close()
	s.Close()
	s, err = New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, ok := s.uploads[u.ID]; !ok {
		t.Fatal("upload lost")
	}
	restored, _ := s.snapshot(state.ID)
	if restored.Status != "failed" || restored.Error == "" {
		t.Fatal(restored)
	}
}
