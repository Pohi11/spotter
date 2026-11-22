package restapi_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/natsevents"
	"github.com/Pohi11/spotter/services/gateway/internal/restapi"
)

const feedCapability = "event-feed"

type feedEvent struct {
	Stream   string          `json:"stream"`
	Subject  string          `json:"subject"`
	Sequence uint64          `json:"sequence"`
	Time     time.Time       `json:"time"`
	Type     string          `json:"type"`
	Payload  json.RawMessage `json:"payload"`
	id       string
}

// feed starts the event feed over a fresh embedded JetStream and returns its URL and a
// JetStream handle for publishing.
func feed(t *testing.T, keepAlive time.Duration) (string, jetstream.JetStream) {
	t.Helper()
	_, js := natstest.Start(t).Connect(t)
	if err := natsjs.Ensure(context.Background(), js); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(restapi.Events(natsevents.New(js), keepAlive))
	t.Cleanup(srv.Close)
	return srv.URL, js
}

func publish(t *testing.T, js jetstream.JetStream, subject, typ string, m proto.Message) {
	t.Helper()
	data, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	msg := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	if typ != "" {
		msg.Header.Set(natsjs.EventTypeHeader, typ)
	}
	if _, err := js.PublishMsg(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
}

// stream connects to the feed and returns a channel of events and of comment lines.
func stream(t *testing.T, url string) (<-chan feedEvent, <-chan string, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d, content type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	events := make(chan feedEvent, 64)
	comments := make(chan string, 64)
	go func() {
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		var id string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, ":"):
				comments <- strings.TrimSpace(line[1:])
			case strings.HasPrefix(line, "id: "):
				id = line[4:]
			case strings.HasPrefix(line, "data: "):
				var ev feedEvent
				if err := json.Unmarshal([]byte(line[6:]), &ev); err == nil {
					ev.id = id
					events <- ev
				}
			}
		}
	}()
	return events, comments, cancel
}

func next(t *testing.T, events <-chan feedEvent) feedEvent {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("no event within 5s")
		return feedEvent{}
	}
}

func TestLiveCompletionIsStreamedDecoded(t *testing.T) {
	spectest.Verifies(t, feedCapability, "The gateway streams platform events as server-sent events")
	url, js := feed(t, time.Minute)
	events, _, _ := stream(t, url)
	publish(t, js, "inference.completed.eurosat-classifier", "spotter.v1.InferenceCompleted", &spotterv1.InferenceCompleted{
		JobId: "job-1", ModelName: "eurosat-classifier", Prediction: &spotterv1.Prediction{Label: "Forest", Confidence: 0.98}})

	ev := next(t, events)
	if ev.Stream != natsjs.InferenceStream || ev.Type != "spotter.v1.InferenceCompleted" || ev.Sequence != 1 || ev.id != "INFERENCE:1" {
		t.Fatalf("event = %+v", ev)
	}
	if ev.Time.IsZero() || ev.Subject != "inference.completed.eurosat-classifier" {
		t.Fatalf("event envelope = %+v", ev)
	}
	var payload struct {
		JobID      string `json:"jobId"`
		Prediction struct {
			Label string `json:"label"`
		} `json:"prediction"`
	}
	if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.JobID != "job-1" || payload.Prediction.Label != "Forest" {
		t.Fatalf("payload = %s (%v)", ev.Payload, err)
	}
}

func TestUnknownTypeKeepsTheEnvelope(t *testing.T) {
	spectest.Verifies(t, feedCapability, "The gateway streams platform events as server-sent events")
	url, js := feed(t, time.Minute)
	events, _, _ := stream(t, url)
	publish(t, js, "registry.events.something_new", "", &spotterv1.Model{Name: "x"})
	publish(t, js, "monitor.drift.other.m", "spotter.v1.NoSuchMessage", &spotterv1.Model{Name: "x"})

	for _, want := range []string{"registry.events.something_new", "monitor.drift.other.m"} {
		ev := next(t, events)
		if ev.Subject != want || string(ev.Payload) != "null" || ev.Sequence == 0 || ev.Time.IsZero() {
			t.Fatalf("event = %+v, payload %s; want %s with a null payload", ev, ev.Payload, want)
		}
	}
}

func TestRecentPastThenLive(t *testing.T) {
	spectest.Verifies(t, feedCapability, "A new client receives the recent past first")
	url, js := feed(t, time.Minute)
	for _, job := range []string{"a", "b", "c"} {
		publish(t, js, "inference.requested.eurosat-classifier", "spotter.v1.InferenceRequested", &spotterv1.InferenceRequested{JobId: job})
	}
	events, _, _ := stream(t, url)
	publish(t, js, "inference.requested.eurosat-classifier", "spotter.v1.InferenceRequested", &spotterv1.InferenceRequested{JobId: "d"})

	for i, want := range []string{"a", "b", "c", "d"} {
		ev := next(t, events)
		var p struct {
			JobID string `json:"jobId"`
		}
		_ = json.Unmarshal(ev.Payload, &p)
		if p.JobID != want || ev.Sequence != uint64(i+1) {
			t.Fatalf("event %d = job %q seq %d, want job %q seq %d", i, p.JobID, ev.Sequence, want, i+1)
		}
	}
}

func TestBadWindowIsRejected(t *testing.T) {
	spectest.Verifies(t, feedCapability, "A new client receives the recent past first")
	url, _ := feed(t, time.Minute)
	for _, since := range []string{"72h", "abc", "-5m", "0s"} {
		resp, err := http.Get(url + "?since=" + since)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "since") {
			t.Errorf("since=%s: status %d body %q; want 400 naming since", since, resp.StatusCode, body)
		}
	}
}

func TestDisconnectRemovesConsumers(t *testing.T) {
	spectest.Verifies(t, feedCapability, "A disconnected client leaves nothing behind")
	url, js := feed(t, time.Minute)
	events, _, cancel := stream(t, url)
	publish(t, js, "inference.requested.eurosat-classifier", "spotter.v1.InferenceRequested", &spotterv1.InferenceRequested{JobId: "a"})
	next(t, events)

	consumers := func() int {
		n := 0
		for _, name := range []string{natsjs.InferenceStream, natsjs.RegistryStream, natsjs.MonitorStream} {
			s, err := js.Stream(context.Background(), name)
			if err != nil {
				t.Fatal(err)
			}
			info, err := s.Info(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			n += info.State.Consumers
		}
		return n
	}
	if n := consumers(); n != 3 {
		t.Fatalf("consumers while connected = %d, want 3 (one per stream)", n)
	}
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for consumers() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("consumers 5s after disconnect = %d, want 0", consumers())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestIdleConnectionGetsKeepAlive(t *testing.T) {
	spectest.Verifies(t, feedCapability, "A disconnected client leaves nothing behind")
	url, _ := feed(t, 100*time.Millisecond)
	_, comments, _ := stream(t, url)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case c := <-comments:
			if c == "keep-alive" {
				return
			}
		case <-deadline:
			t.Fatal("no keep-alive comment within 3s")
		}
	}
}
