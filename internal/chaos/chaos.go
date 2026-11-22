// Package chaos runs closed-loop load while it injects one fault and heals it, and turns
// what happened to each request into a report (spec: failure-lab). The fault, the client,
// and the completion source are interfaces, so the scheduling and the arithmetic are
// tested without a platform.
package chaos

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

// Fault is one kind of failure. Heal undoes Inject; for a fault that heals itself (a
// restart), Heal does nothing.
type Fault interface {
	Inject(ctx context.Context) error
	Heal(ctx context.Context) error
}

// Client sends one request. A synchronous client returns done=true when the result came
// back in the call; an asynchronous client returns the accepted job's id, whose completion
// arrives on the completions channel. err means the request was rejected.
type Client interface {
	Send(ctx context.Context, image []byte) (jobID string, done bool, err error)
}

// Completion is one stored completion event for a job.
type Completion struct {
	JobID string
	At    time.Time
}

// Config controls a run.
type Config struct {
	Concurrency int
	Duration    time.Duration // how long clients send
	FaultAt     time.Duration // offset of Inject
	HealAt      time.Duration // offset of Heal
	JobWait     time.Duration // how long a client waits for its job before sending the next
	Drain       time.Duration // how long to wait for stragglers after Duration
	Images      [][]byte
}

// Record is one request and what happened to it, in offsets from the run's start.
type Record struct {
	Sent        time.Duration
	Rejected    bool
	JobID       string        // asynchronous only
	Completed   time.Duration // first completion; 0 if none
	Completions int
}

// Events are the fault's and heal's actual offsets and errors.
type Events struct {
	InjectedAt, HealedAt time.Duration
	InjectErr, HealErr   string
}

// Run drives the load and the fault, and returns every request's record.
func Run(ctx context.Context, cfg Config, client Client, completions <-chan Completion, fault Fault) ([]Record, Events) {
	start := time.Now()
	since := func() time.Duration { return time.Since(start) }

	var mu sync.Mutex
	var records []*Record
	byJob := map[string]*Record{}
	waiting := map[string]chan struct{}{}
	early := map[string][]time.Duration{} // completions seen before Send returned the id

	// Completions.
	collectDone := make(chan struct{})
	collectCtx, stopCollect := context.WithCancel(context.Background())
	go func() {
		defer close(collectDone)
		for {
			select {
			case c := <-completions:
				at := c.At.Sub(start)
				mu.Lock()
				if r, ok := byJob[c.JobID]; ok {
					r.Completions++
					if r.Completions == 1 {
						r.Completed = at
					}
					if ch, ok := waiting[c.JobID]; ok {
						close(ch)
						delete(waiting, c.JobID)
					}
				} else {
					early[c.JobID] = append(early[c.JobID], at)
				}
				mu.Unlock()
			case <-collectCtx.Done():
				return
			}
		}
	}()

	// The fault.
	var ev Events
	faultDone := make(chan struct{})
	go func() {
		defer close(faultDone)
		sleepUntil(ctx, start.Add(cfg.FaultAt))
		ev.InjectedAt = since()
		if err := fault.Inject(ctx); err != nil {
			ev.InjectErr = err.Error()
		}
		sleepUntil(ctx, start.Add(cfg.HealAt))
		ev.HealedAt = since()
		if err := fault.Heal(ctx); err != nil {
			ev.HealErr = err.Error()
		}
	}()

	// Clients.
	var wg sync.WaitGroup
	for c := range cfg.Concurrency {
		wg.Go(func() {
			for i := c; since() < cfg.Duration && ctx.Err() == nil; i += cfg.Concurrency {
				r := &Record{Sent: since()}
				jobID, done, err := client.Send(ctx, cfg.Images[i%len(cfg.Images)])
				mu.Lock()
				records = append(records, r)
				switch {
				case err != nil:
					r.Rejected = true
					mu.Unlock()
					time.Sleep(50 * time.Millisecond) // a rejected client backs off briefly
					continue
				case done:
					r.Completions, r.Completed = 1, since()
					mu.Unlock()
					continue
				}
				r.JobID = jobID
				byJob[jobID] = r
				if ats := early[jobID]; len(ats) > 0 {
					r.Completions, r.Completed = len(ats), ats[0]
					delete(early, jobID)
					mu.Unlock()
					continue
				}
				ch := make(chan struct{})
				waiting[jobID] = ch
				mu.Unlock()
				select {
				case <-ch:
				case <-time.After(cfg.JobWait):
				case <-ctx.Done():
				}
			}
		})
	}
	wg.Wait()
	<-faultDone

	// Stragglers: wait for every accepted job, up to Drain.
	deadline := time.Now().Add(cfg.Drain)
	for time.Now().Before(deadline) {
		mu.Lock()
		pending := 0
		for _, r := range records {
			if r.JobID != "" && r.Completions == 0 {
				pending++
			}
		}
		mu.Unlock()
		if pending == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Late duplicates of the last jobs would arrive right after; give them a moment.
	time.Sleep(time.Second)
	stopCollect()
	<-collectDone

	mu.Lock()
	defer mu.Unlock()
	out := make([]Record, len(records))
	for i, r := range records {
		out[i] = *r
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sent < out[j].Sent })
	return out, ev
}

func sleepUntil(ctx context.Context, t time.Time) {
	select {
	case <-time.After(time.Until(t)):
	case <-ctx.Done():
	}
}

// Report summarizes a run (spec: failure-lab "Each run reports what clients and the
// system experienced").
type Report struct {
	Requests     int      `json:"requests"`
	Rejected     int      `json:"rejected"`
	Completed    int      `json:"completed"`
	Lost         int      `json:"lost"`
	Duplicates   int      `json:"duplicates"`
	P50ms        float64  `json:"p50_ms"`
	P99ms        float64  `json:"p99_ms"`
	MaxMs        float64  `json:"max_ms"`
	BaselinePerS float64  `json:"baseline_per_second"`
	RecoveryS    *float64 `json:"recovery_seconds"` // nil: did not recover
	// Per second from the run's start: completions (first completion of a job, or a
	// synchronous result) and rejections.
	CompletedPerS []int `json:"completed_per_second"`
	RejectedPerS  []int `json:"rejected_per_second"`
}

// Summarize computes the report. Recovery runs from healAt to the end of the first
// one-second bucket whose completions reach 80% of the mean per-second completions
// between the first second and faultAt.
func Summarize(records []Record, faultAt, healAt, length time.Duration) Report {
	secs := int(math.Ceil(length.Seconds()))
	rep := Report{CompletedPerS: make([]int, secs), RejectedPerS: make([]int, secs)}
	var latencies []float64
	bucket := func(d time.Duration) int { return min(int(d/time.Second), secs-1) }
	for _, r := range records {
		rep.Requests++
		switch {
		case r.Rejected:
			rep.Rejected++
			rep.RejectedPerS[bucket(r.Sent)]++
		case r.Completions == 0:
			rep.Lost++
		default:
			rep.Completed++
			if r.Completions > 1 {
				rep.Duplicates++
			}
			latencies = append(latencies, float64(r.Completed-r.Sent)/float64(time.Millisecond))
			rep.CompletedPerS[bucket(r.Completed)]++
		}
	}
	sort.Float64s(latencies)
	rep.P50ms, rep.P99ms = percentile(latencies, 50), percentile(latencies, 99)
	if n := len(latencies); n > 0 {
		rep.MaxMs = latencies[n-1]
	}

	first, last := 1, int(faultAt/time.Second) // skip the first second: clients are starting
	if last > first {
		sum := 0
		for _, c := range rep.CompletedPerS[first:last] {
			sum += c
		}
		rep.BaselinePerS = float64(sum) / float64(last-first)
	}
	h := int(healAt / time.Second)
	for k := h; k < secs; k++ {
		if rep.BaselinePerS > 0 && float64(rep.CompletedPerS[k]) >= 0.8*rep.BaselinePerS {
			s := float64(k+1) - healAt.Seconds()
			rep.RecoveryS = &s
			break
		}
	}
	return rep
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo, hi := int(math.Floor(idx)), int(math.Ceil(idx))
	return sorted[lo] + (sorted[hi]-sorted[lo])*(idx-float64(lo))
}
