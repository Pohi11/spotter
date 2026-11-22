// Package loadgen drives inference load and measures end-to-end latency from completion
// events (the load-testing capability).
package loadgen

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

// Submitter submits one image and returns its job id.
type Submitter interface {
	Submit(ctx context.Context, image []byte) (jobID string, err error)
}

// Completion is a job's completion event and when it arrived.
type Completion struct {
	JobID string
	At    time.Time
}

// Config controls a run.
type Config struct {
	Concurrency int           // concurrent clients
	Duration    time.Duration // how long clients keep submitting
	Timeout     time.Duration // how long to wait for outstanding completions afterwards
	Images      [][]byte      // submitted round-robin
}

// Report is what a run measured.
type Report struct {
	Submitted         int     `json:"submitted"`
	FailedSubmissions int     `json:"failed_submissions"`
	Completed         int     `json:"completed"`
	Lost              int     `json:"lost"`
	Seconds           float64 `json:"seconds"`
	PerSecond         float64 `json:"completed_per_second"`
	P50ms             float64 `json:"p50_ms"`
	P95ms             float64 `json:"p95_ms"`
	P99ms             float64 `json:"p99_ms"`
	MaxMs             float64 `json:"max_ms"`
}

// Run submits from cfg.Concurrency goroutines for cfg.Duration, then waits up to
// cfg.Timeout for outstanding completions. A completion can arrive before Submit has
// returned its job id; such completions are held until the id is known.
func Run(ctx context.Context, cfg Config, sub Submitter, completions <-chan Completion) Report {
	var mu sync.Mutex
	started := map[string]time.Time{} // submitted, not yet completed
	early := map[string]time.Time{}   // completed before Submit returned
	var latencies []float64
	rep := Report{}

	record := func(start, end time.Time) {
		latencies = append(latencies, float64(end.Sub(start))/float64(time.Millisecond))
	}

	collectorDone := make(chan struct{})
	stopCollecting := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for {
			select {
			case c := <-completions:
				mu.Lock()
				if start, ok := started[c.JobID]; ok {
					record(start, c.At)
					delete(started, c.JobID)
					rep.Completed++
				} else {
					early[c.JobID] = c.At
				}
				mu.Unlock()
			case <-stopCollecting:
				return
			}
		}
	}()

	begin := time.Now()
	deadline := begin.Add(cfg.Duration)
	var wg sync.WaitGroup
	for w := range cfg.Concurrency {
		wg.Go(func() {
			for i := w; time.Now().Before(deadline) && ctx.Err() == nil; i += cfg.Concurrency {
				image := cfg.Images[i%len(cfg.Images)]
				start := time.Now()
				id, err := sub.Submit(ctx, image)
				mu.Lock()
				rep.Submitted++
				switch {
				case err != nil:
					rep.FailedSubmissions++
				default:
					if end, ok := early[id]; ok {
						record(start, end)
						delete(early, id)
						rep.Completed++
					} else {
						started[id] = start
					}
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	// Wait for the stragglers, then count what never arrived.
	waitUntil := time.Now().Add(cfg.Timeout)
	for time.Now().Before(waitUntil) {
		mu.Lock()
		outstanding := len(started)
		mu.Unlock()
		if outstanding == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(stopCollecting)
	<-collectorDone

	mu.Lock()
	defer mu.Unlock()
	rep.Lost = len(started)
	rep.Seconds = time.Since(begin).Seconds()
	if rep.Seconds > 0 {
		rep.PerSecond = float64(rep.Completed) / rep.Seconds
	}
	rep.P50ms, rep.P95ms, rep.P99ms = Percentile(latencies, 50), Percentile(latencies, 95), Percentile(latencies, 99)
	if len(latencies) > 0 {
		rep.MaxMs = slicesMax(latencies)
	}
	return rep
}

// Percentile with linear interpolation between closest ranks (numpy's default).
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	rank := p / 100 * float64(len(s)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	return s[lo] + (s[hi]-s[lo])*(rank-float64(lo))
}

func slicesMax(v []float64) float64 {
	m := v[0]
	for _, x := range v[1:] {
		m = max(m, x)
	}
	return m
}
