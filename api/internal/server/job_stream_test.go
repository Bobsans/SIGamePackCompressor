package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"sigamepackcompressor/internal/siq"
)

func readUpdate(t *testing.T, ctx context.Context, conn *websocket.Conn) JobUpdate {
	t.Helper()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var event JobUpdate
	if err = json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func TestJobWebSocketStreamsIndividualRecordsAndReconnects(t *testing.T) {
	s, ts, _ := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	jobCtx, jobCancel := context.WithCancel(s.ctx)
	defer jobCancel()
	j := &job{state: JobState{ID: identifier(), Status: "running", Total: 2}, ctx: jobCtx, cancel: jobCancel}
	s.mu.Lock()
	s.jobs[j.state.ID] = j
	s.mu.Unlock()
	endpoint := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/jobs/" + j.state.ID + "/ws"
	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if event := readUpdate(t, ctx, conn); event.Type != "snapshot" || event.State.ID != j.state.ID {
		t.Fatal(event)
	}
	m := siq.Media{Name: "Images/one.png", Type: "image", Size: 100}
	transform := s.withFileLog(j, func(context.Context, siq.Media, io.Reader) (string, string, error) { return "", "", nil })
	transform(ctx, m, strings.NewReader(""))
	if event := readUpdate(t, ctx, conn); event.Type != "file" || event.Entry.Status != "processing" || event.Entry.Sequence != 1 {
		t.Fatal(event)
	}
	if event := readUpdate(t, ctx, conn); event.Type != "status" || len(event.State.Log) != 0 {
		t.Fatal("incremental status carried full log")
	}
	s.mu.Lock()
	j.state.Progress++
	completeFileLog(&j.state, m, 50, "")
	s.persist(j)
	s.mu.Unlock()
	if event := readUpdate(t, ctx, conn); event.Type != "file" || event.Entry.Status != "compressed" || *event.Entry.NewSize != 50 {
		t.Fatal(event)
	}
	if event := readUpdate(t, ctx, conn); event.Type != "status" || event.State.Progress != 1 {
		t.Fatal(event)
	}
	conn.CloseNow()
	s.mu.Lock()
	j.state.Progress++
	completeFileLog(&j.state, siq.Media{Name: "Images/two.png", Type: "image", Size: 80}, 80, "")
	s.persist(j)
	s.mu.Unlock()
	reconnected, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.CloseNow()
	event := readUpdate(t, ctx, reconnected)
	if event.Type != "snapshot" || len(event.State.Log) != 2 || event.State.Progress != 2 {
		t.Fatal("reconnection did not restore full history")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/jobs/"+j.state.ID+"/cancel", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	event = readUpdate(t, ctx, reconnected)
	if event.Type != "status" || event.State.Status != "cancelled" {
		t.Fatal(event)
	}
	if _, _, err = reconnected.Read(ctx); websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatal(err)
	}
}

func TestJobWebSocketOriginAndMissingJob(t *testing.T) {
	s, ts, _ := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/jobs/" + identifier() + "/ws"
	_, response, err := websocket.Dial(ctx, url, nil)
	if err == nil || response.StatusCode != 404 {
		t.Fatal("missing job accepted")
	}
	j := &job{state: JobState{ID: identifier(), Status: "completed"}}
	s.mu.Lock()
	s.jobs[j.state.ID] = j
	s.mu.Unlock()
	url = "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/jobs/" + j.state.ID + "/ws"
	_, response, err = websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://untrusted.example"}}})
	if err == nil || response.StatusCode != 403 {
		t.Fatal("foreign origin accepted")
	}
}

func TestSlowJobStreamReconnectsInsteadOfDroppingEvents(t *testing.T) {
	s, _, _ := setup(t)
	ch := make(chan JobUpdate, 1)
	j := &job{subscribers: map[chan JobUpdate]struct{}{ch: {}}}
	s.mu.Lock()
	s.publish(j, JobUpdate{Type: "file"})
	s.publish(j, JobUpdate{Type: "status"})
	s.mu.Unlock()
	if len(j.subscribers) != 0 {
		t.Fatal("slow stream retained")
	}
	<-ch
	if _, ok := <-ch; ok {
		t.Fatal("slow stream not closed")
	}
}
