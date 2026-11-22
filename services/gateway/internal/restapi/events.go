package restapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Pohi11/spotter/services/gateway/internal/domain"
	"github.com/Pohi11/spotter/services/gateway/internal/ports"
)

// Event feed limits (spec: event-feed).
const (
	DefaultSince = 10 * time.Minute
	MaxSince     = 24 * time.Hour
	KeepAlive    = 15 * time.Second
)

type wireEvent struct {
	domain.Event
	Payload json.RawMessage `json:"payload"`
}

// Events serves GET /v1/events: the recent past, then live events, as server-sent
// events, with a keep-alive comment every keepAlive while idle.
func Events(src ports.EventSource, keepAlive time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since := DefaultSince
		if v := r.URL.Query().Get("since"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 || d > MaxSince {
				http.Error(w, fmt.Sprintf("since: want a duration above 0 and at most %s, got %q", MaxSince, v), http.StatusBadRequest)
				return
			}
			since = d
		}
		streams := src.Streams()
		if v := r.URL.Query().Get("streams"); v != "" {
			known := map[string]bool{}
			for _, s := range streams {
				known[s] = true
			}
			streams = nil
			for _, name := range strings.Split(v, ",") {
				if !known[name] {
					http.Error(w, fmt.Sprintf("streams: unknown stream %q; want a comma-separated subset of %s", name, strings.Join(src.Streams(), ",")), http.StatusBadRequest)
					return
				}
				streams = append(streams, name)
			}
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		events, err := src.Events(r.Context(), since, streams)
		if err != nil {
			http.Error(w, "event streams unavailable", http.StatusServiceUnavailable)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no") // tell buffering proxies to pass events through
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()

		tick := time.NewTicker(keepAlive)
		defer tick.Stop()
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					return
				}
				payload := json.RawMessage("null")
				if ev.Payload != nil {
					payload = ev.Payload
				}
				data, err := json.Marshal(wireEvent{Event: ev, Payload: payload})
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "id: %s:%d\ndata: %s\n\n", ev.Stream, ev.Sequence, data); err != nil {
					return
				}
				flusher.Flush()
				tick.Reset(keepAlive)
			case <-tick.C:
				if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
}
