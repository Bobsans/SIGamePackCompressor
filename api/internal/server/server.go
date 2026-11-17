package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sigamepackcompressor/internal/config"
	"sigamepackcompressor/internal/media"
	"sigamepackcompressor/internal/siq"
)

type Upload struct {
	ID        string                   `json:"id"`
	Filename  string                   `json:"filename"`
	CreatedAt time.Time                `json:"created_at"`
	Analysis  siq.Analysis             `json:"analysis"`
	Details   map[string]media.Details `json:"media_details"`
}
type JobState struct {
	ID           string         `json:"id"`
	UploadID     string         `json:"upload_id"`
	Status       string         `json:"status"`
	Options      media.Options  `json:"options"`
	Progress     int            `json:"progress"`
	Total        int            `json:"total"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Error        string         `json:"error,omitempty"`
	Warnings     []string       `json:"warnings"`
	DownloadURL  string         `json:"download_url,omitempty"`
	OriginalSize int64          `json:"original_size"`
	ResultSize   int64          `json:"result_size,omitempty"`
	Log          []FileLogEntry `json:"log,omitempty"`
	RemovedFiles int            `json:"removed_files,omitempty"`
	RemovedBytes int64          `json:"removed_bytes,omitempty"`
}
type job struct {
	state             JobState
	cancel            context.CancelFunc
	ctx               context.Context
	subscribers       map[chan JobUpdate]struct{}
	lastPublishedFile *FileLogEntry
}
type Server struct {
	cfg       config.Config
	processor media.Processor
	store     *store
	mu        sync.Mutex
	uploads   map[string]Upload
	jobs      map[string]*job
	queue     chan *job
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	// Bounds simultaneous upload analysis and temporary disk usage.
	uploadSlots chan struct{}
}

func New(cfg config.Config) (*Server, error) {
	for _, name := range []string{"uploads", "results", "tmp"} {
		if err := os.MkdirAll(filepath.Join(cfg.StoragePath, name), 0700); err != nil {
			return nil, err
		}
	}
	st, err := openStore(cfg.StoragePath)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{cfg: cfg, processor: cfg.Processor(), store: st, uploads: map[string]Upload{}, jobs: map[string]*job{}, queue: make(chan *job, cfg.QueueSize), ctx: ctx, cancel: cancel, uploadSlots: make(chan struct{}, cfg.Workers+1)}
	if err := s.restore(); err != nil {
		cancel()
		st.db.Close()
		return nil, err
	}
	for i := 0; i < cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
	return s, nil
}
func (s *Server) Close() error { s.cancel(); s.wg.Wait(); return s.store.db.Close() }
func (s *Server) restore() error {
	rows, err := s.store.load("go_uploads")
	if err != nil {
		return err
	}
	for _, b := range rows {
		var u Upload
		if err = json.Unmarshal(b, &u); err != nil {
			return err
		}
		s.uploads[u.ID] = u
	}
	rows, err = s.store.load("go_jobs")
	if err != nil {
		return err
	}
	for _, b := range rows {
		var state JobState
		if err = json.Unmarshal(b, &state); err != nil {
			return err
		}
		storedLog, err := s.store.loadJobLog(state.ID)
		if err != nil {
			return err
		}
		state.Log = storedLog
		if state.Status == "running" || state.Status == "queued" {
			state.Status = "failed"
			interruptFileLog(&state)
			state.Error = "Server restarted; submit a new job using the saved upload"
			state.UpdatedAt = time.Now().UTC()
			if err = s.store.saveJob(state); err != nil {
				return err
			}
		}
		s.jobs[state.ID] = &job{state: state}
	}
	return nil
}
func identifier() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func jsonReply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, err error) {
	jsonReply(w, status, map[string]string{"error": err.Error(), "detail": err.Error()})
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		jsonReply(w, 200, map[string]any{"status": "ok", "version": s.cfg.BuildVersion, "tools": s.processor.Capabilities()})
	})
	mux.HandleFunc("GET /api/options", func(w http.ResponseWriter, r *http.Request) {
		jsonReply(w, 200, map[string]any{"defaults": media.Defaults(), "input_versions": []string{"3", "3.1", "4", "5"}, "output_versions": []string{"5", "source"}, "image_formats": []string{"jpeg", "png", "gif", "webp", "avif"}, "tools": s.processor.Capabilities(), "max_upload_bytes": s.cfg.MaxUploadMB << 20})
	})
	mux.HandleFunc("POST /api/uploads", s.uploadHandler)
	mux.HandleFunc("GET /api/uploads/{id}", s.getUpload)
	mux.HandleFunc("POST /api/uploads/{id}/jobs", s.createJob)
	mux.HandleFunc("GET /api/jobs/{id}", s.getJob)
	mux.HandleFunc("GET /api/jobs/{id}/ws", s.jobWS)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.cancelJob)
	mux.HandleFunc("GET /api/jobs/{id}/download", s.downloadJob)
	if s.cfg.WebPath != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(s.cfg.WebPath)))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !s.allowedOrigin(r, origin) {
				fail(w, 403, fmt.Errorf("origin not allowed"))
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) allowedOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	for _, allowed := range s.cfg.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (s *Server) receive(w http.ResponseWriter, r *http.Request) (Upload, error) {
	select {
	case s.uploadSlots <- struct{}{}:
		defer func() { <-s.uploadSlots }()
	default:
		return Upload{}, fmt.Errorf("upload capacity reached; try again later")
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadMB<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		return Upload{}, fmt.Errorf("expected multipart form: %w", err)
	}
	var upload Upload
	for {
		part, e := mr.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			return Upload{}, e
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		if upload.ID != "" {
			part.Close()
			return Upload{}, fmt.Errorf("only one file is accepted")
		}
		name := strings.ReplaceAll(part.FileName(), "\\", "/")
		name = filepath.Base(name)
		if !strings.EqualFold(filepath.Ext(name), ".siq") {
			part.Close()
			return Upload{}, fmt.Errorf("file must have a .siq extension")
		}
		id := identifier()
		temp, err := os.CreateTemp(filepath.Join(s.cfg.StoragePath, "tmp"), "upload-*")
		if err != nil {
			return Upload{}, err
		}
		tmp := temp.Name()
		defer os.Remove(tmp)
		_, err = io.Copy(temp, part)
		ce := temp.Close()
		part.Close()
		if err != nil {
			return Upload{}, err
		}
		if ce != nil {
			return Upload{}, ce
		}
		p, err := siq.Open(tmp, s.cfg.Limits)
		if err != nil {
			return Upload{}, err
		}
		upload = Upload{id, name, time.Now().UTC(), p.Info, map[string]media.Details{}}
		// Probe embedded resources only. External URLs are never fetched.
		probeCtx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		for _, m := range p.Info.Media {
			if probeCtx.Err() != nil {
				upload.Analysis.Warnings = append(upload.Analysis.Warnings, "Media analysis timed out")
				break
			}
			if m.Type == "html" {
				continue
			}
			file, err := p.Archive.Open(m.Name)
			if err != nil {
				continue
			}
			probe, err := os.CreateTemp(filepath.Join(s.cfg.StoragePath, "tmp"), "probe-*")
			if err != nil {
				file.Close()
				cancel()
				p.Close()
				return Upload{}, err
			}
			probeName := probe.Name()
			_, copyErr := io.Copy(probe, file)
			probe.Close()
			file.Close()
			if copyErr == nil {
				d, probeErr := s.processor.Inspect(probeCtx, m.Type, probeName)
				if probeErr == nil {
					upload.Details[m.Name] = d
				} else {
					upload.Analysis.Warnings = append(upload.Analysis.Warnings, "Could not inspect "+m.Name+": "+probeErr.Error())
				}
			}
			os.Remove(probeName)
		}
		cancel()
		p.Close()
		if r.Context().Err() != nil {
			return Upload{}, r.Context().Err()
		}
		final := filepath.Join(s.cfg.StoragePath, "uploads", id+".siq")
		if err = os.Rename(tmp, final); err != nil {
			return Upload{}, err
		}
		// Consume the remaining form before publishing metadata to catch truncated uploads.
		for {
			next, e := mr.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				os.Remove(final)
				return Upload{}, e
			}
			duplicate := next.FormName() == "file"
			_, e = io.Copy(io.Discard, next)
			next.Close()
			if e != nil || duplicate {
				os.Remove(final)
				return Upload{}, fmt.Errorf("invalid multipart upload")
			}
		}
		s.mu.Lock()
		err = s.store.save("go_uploads", id, upload)
		if err == nil {
			s.uploads[id] = upload
		}
		s.mu.Unlock()
		if err != nil {
			os.Remove(final)
			return Upload{}, err
		}
		return upload, nil
	}
	return Upload{}, fmt.Errorf("file field is required")
}
func (s *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	u, err := s.receive(w, r)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			fail(w, 413, err)
		} else {
			fail(w, 400, err)
		}
		return
	}
	jsonReply(w, 201, u)
}
func (s *Server) getUpload(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	u, ok := s.uploads[r.PathValue("id")]
	s.mu.Unlock()
	if !ok {
		fail(w, 404, fmt.Errorf("upload not found"))
		return
	}
	jsonReply(w, 200, u)
}

func (s *Server) submit(uploadID string, options media.Options) (JobState, error) {
	if err := options.Validate(); err != nil {
		return JobState{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.uploads[uploadID]
	if !ok {
		return JobState{}, fmt.Errorf("upload not found")
	}
	if err := s.processor.Check(options, u.Analysis.Media); err != nil {
		return JobState{}, err
	}
	if len(s.queue) >= cap(s.queue) {
		return JobState{}, fmt.Errorf("job queue is full")
	}
	ctx, cancel := context.WithCancel(s.ctx)
	now := time.Now().UTC()
	state := JobState{ID: identifier(), UploadID: uploadID, Status: "queued", Options: options, Total: len(u.Analysis.Media), CreatedAt: now, UpdatedAt: now, Warnings: []string{}, OriginalSize: u.Analysis.Size}
	j := &job{state: state, cancel: cancel, ctx: ctx}
	if err := s.store.saveJob(state); err != nil {
		cancel()
		return JobState{}, err
	}
	s.jobs[state.ID] = j
	s.queue <- j
	return state, nil
}
func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	o := media.Defaults()
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&o); err != nil && err != io.EOF {
		fail(w, 400, err)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		fail(w, 400, fmt.Errorf("expected a single JSON object"))
		return
	}
	state, err := s.submit(r.PathValue("id"), o)
	if err != nil {
		fail(w, 422, err)
		return
	}
	jsonReply(w, 202, state)
}
func (s *Server) snapshot(id string) (JobState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return JobState{}, false
	}
	v := copyJobState(j.state, true)
	return v, true
}
func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	state, ok := s.snapshot(r.PathValue("id"))
	if !ok {
		fail(w, 404, fmt.Errorf("job not found"))
		return
	}
	jsonReply(w, 200, state)
}
func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	j, ok := s.jobs[r.PathValue("id")]
	if !ok {
		s.mu.Unlock()
		fail(w, 404, fmt.Errorf("job not found"))
		return
	}
	if !terminal(j.state.Status) {
		j.cancel()
		j.state.Status = "cancelled"
		interruptFileLog(&j.state)
		j.state.UpdatedAt = time.Now().UTC()
		if err := s.store.saveJob(j.state); err != nil {
			s.mu.Unlock()
			fail(w, 500, err)
			return
		}
		s.publishJob(j)
	}
	state := copyJobState(j.state, true)
	s.mu.Unlock()
	jsonReply(w, 200, state)
}
func terminal(status string) bool {
	return status == "completed" || status == "failed" || status == "cancelled"
}
func (s *Server) persist(j *job) {
	j.state.UpdatedAt = time.Now().UTC()
	if err := s.store.saveJob(j.state); err != nil {
		slog.Error("persist job", "id", j.state.ID, "error", err)
		j.state.Error = "Could not persist job state"
	}
	s.publishJob(j)
}
func (s *Server) worker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case j := <-s.queue:
			s.run(j)
		}
	}
}
func (s *Server) run(j *job) {
	defer j.cancel()
	s.mu.Lock()
	if j.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	j.state.Status = "running"
	s.persist(j)
	u := s.uploads[j.state.UploadID]
	o := j.state.Options
	id := j.state.ID
	s.mu.Unlock()
	work, err := os.MkdirTemp(filepath.Join(s.cfg.StoragePath, "tmp"), "job-*")
	if err != nil {
		s.finish(j, err, 0)
		return
	}
	defer os.RemoveAll(work)
	p, err := siq.Open(filepath.Join(s.cfg.StoragePath, "uploads", u.ID+".siq"), s.cfg.Limits)
	if err != nil {
		s.finish(j, err, 0)
		return
	}
	defer p.Close()
	if err = p.Prepare(o.OutputVersion); err != nil {
		s.finish(j, err, 0)
		return
	}
	s.mu.Lock()
	j.state.Total = len(p.Info.Media)
	j.state.Warnings = append(j.state.Warnings, p.Info.Warnings...)
	s.persist(j)
	s.mu.Unlock()
	out := filepath.Join(work, "result.siq")
	err = p.Write(j.ctx, out, o.OutputVersion, s.withFileLog(j, s.processor.Transform(work, o)), func(m siq.Media, name string, size int64, warning string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if terminal(j.state.Status) {
			return
		}
		j.state.Progress++
		completeFileLog(&j.state, m, size, warning)
		if warning != "" {
			j.state.Warnings = append(j.state.Warnings, m.Name+": "+warning)
		}
		s.persist(j)
	})
	if err != nil {
		s.finish(j, err, 0)
		return
	}
	st, err := os.Stat(out)
	if err != nil {
		s.finish(j, err, 0)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if j.ctx.Err() != nil {
		j.state.Status = "cancelled"
		s.persist(j)
		return
	}
	if err = os.Rename(out, filepath.Join(s.cfg.StoragePath, "results", id+".siq")); err != nil {
		j.state.Status = "failed"
		j.state.Error = err.Error()
		s.persist(j)
		return
	}
	j.state.Status = "completed"
	j.state.Progress = j.state.Total
	j.state.ResultSize = st.Size()
	j.state.RemovedFiles = p.Info.UnusedFiles
	j.state.RemovedBytes = p.Info.UnusedSize
	j.state.DownloadURL = "/api/jobs/" + id + "/download"
	s.persist(j)
}
func (s *Server) finish(j *job, err error, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	interruptFileLog(&j.state)
	if j.ctx.Err() != nil {
		j.state.Status = "cancelled"
	} else {
		j.state.Status = "failed"
		j.state.Error = err.Error()
	}
	s.persist(j)
}
func (s *Server) downloadJob(w http.ResponseWriter, r *http.Request) {
	s.serveJob(w, r, r.PathValue("id"))
}
func (s *Server) serveJob(w http.ResponseWriter, r *http.Request, id string) {
	state, ok := s.snapshot(id)
	if !ok {
		fail(w, 404, fmt.Errorf("job not found"))
		return
	}
	if state.Status != "completed" {
		fail(w, 409, fmt.Errorf("result is not ready"))
		return
	}
	s.mu.Lock()
	u := s.uploads[state.UploadID]
	s.mu.Unlock()
	name := strings.TrimSuffix(u.Filename, filepath.Ext(u.Filename)) + "-compressed.siq"
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filepath.Join(s.cfg.StoragePath, "results", id+".siq"))
}
