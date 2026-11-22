package loadgen_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Pohi11/spotter/internal/loadgen"
	"github.com/Pohi11/spotter/internal/spectest"
)

const requirement = "The load generator measures end-to-end latency from completion events"

// fakeSystem completes jobs after `delay`, except every `loseEvery`th job (never completes)
// and every `failEvery`th submission (rejected). Zero disables either.
type fakeSystem struct {
	n                    atomic.Int64
	delay                time.Duration
	loseEvery, failEvery int64
	completions          chan loadgen.Completion
	immediate            bool // complete before Submit returns
}

func (f *fakeSystem) Submit(ctx context.Context, _ []byte) (string, error) {
	n := f.n.Add(1)
	if f.failEvery > 0 && n%f.failEvery == 0 {
		return "", errors.New("unavailable")
	}
	id := fmt.Sprintf("job-%d", n)
	if f.loseEvery > 0 && n%f.loseEvery == 0 {
		return id, nil
	}
	if f.immediate {
		f.completions <- loadgen.Completion{JobID: id, At: time.Now()}
		return id, nil
	}
	go func() {
		time.Sleep(f.delay)
		f.completions <- loadgen.Completion{JobID: id, At: time.Now()}
	}()
	time.Sleep(time.Millisecond) // pace submissions
	return id, nil
}

func run(f *fakeSystem) loadgen.Report {
	return loadgen.Run(context.Background(), loadgen.Config{
		Concurrency: 4, Duration: 200 * time.Millisecond, Timeout: 300 * time.Millisecond, Images: [][]byte{{1}},
	}, f, f.completions)
}

func TestEveryJobCompletes(t *testing.T) {
	spectest.Verifies(t, "load-testing", requirement)
	f := &fakeSystem{delay: 20 * time.Millisecond, completions: make(chan loadgen.Completion, 10000)}
	r := run(f)
	if r.Submitted == 0 || r.Completed != r.Submitted || r.Lost != 0 || r.FailedSubmissions != 0 {
		t.Fatalf("report = %+v", r)
	}
	if r.P50ms < 15 || r.P50ms > 200 || r.P99ms < r.P50ms || r.MaxMs < r.P99ms || r.PerSecond <= 0 {
		t.Fatalf("implausible latencies: %+v", r)
	}
}

func TestCompletionBeforeSubmitReturns(t *testing.T) {
	spectest.Verifies(t, "load-testing", requirement)
	f := &fakeSystem{immediate: true, completions: make(chan loadgen.Completion, 10000)}
	r := run(f)
	if r.Completed != r.Submitted || r.Lost != 0 {
		t.Fatalf("early completions were not matched: %+v", r)
	}
}

func TestSomeJobsNeverComplete(t *testing.T) {
	spectest.Verifies(t, "load-testing", requirement)
	f := &fakeSystem{delay: 10 * time.Millisecond, loseEvery: 5, completions: make(chan loadgen.Completion, 10000)}
	r := run(f)
	if r.Lost == 0 || r.Completed+r.Lost != r.Submitted {
		t.Fatalf("report = %+v; want lost jobs counted separately", r)
	}
	if r.MaxMs > 250 { // lost jobs must not appear in the percentiles
		t.Fatalf("max latency %v ms includes lost jobs", r.MaxMs)
	}
}

func TestSubmissionErrors(t *testing.T) {
	spectest.Verifies(t, "load-testing", requirement)
	f := &fakeSystem{delay: 10 * time.Millisecond, failEvery: 3, completions: make(chan loadgen.Completion, 10000)}
	r := run(f)
	if r.FailedSubmissions == 0 || r.Completed+r.FailedSubmissions != r.Submitted {
		t.Fatalf("report = %+v", r)
	}
}

func TestPercentileInterpolates(t *testing.T) {
	v := make([]float64, 100)
	for i := range v {
		v[i] = float64(i + 1)
	}
	for p, want := range map[float64]float64{50: 50.5, 95: 95.05, 99: 99.01} {
		if got := loadgen.Percentile(v, p); math.Abs(got-want) > 1e-9 {
			t.Errorf("p%v = %v, want %v", p, got, want)
		}
	}
}

func TestClosedLoopBoundsOutstandingJobs(t *testing.T) {
	spectest.Verifies(t, "load-testing", "Closed-loop mode bounds outstanding jobs")
	f := &fakeSystem{delay: 15 * time.Millisecond, completions: make(chan loadgen.Completion, 10000)}
	r := loadgen.Run(context.Background(), loadgen.Config{
		Concurrency: 4, Duration: 200 * time.Millisecond, Timeout: time.Second, Images: [][]byte{{1}}, ClosedLoop: true,
	}, f, f.completions)
	if r.MaxOutstanding > 4 || r.MaxOutstanding == 0 {
		t.Fatalf("max outstanding = %d, want 1..4", r.MaxOutstanding)
	}
	if r.Completed != r.Submitted || r.Lost != 0 {
		t.Fatalf("report = %+v", r)
	}
}

func TestOpenLoopCanExceedTheClientCount(t *testing.T) {
	f := &fakeSystem{delay: 50 * time.Millisecond, completions: make(chan loadgen.Completion, 10000)}
	r := run(f)
	if r.MaxOutstanding <= 4 {
		t.Fatalf("open loop max outstanding = %d; expected a queue to build", r.MaxOutstanding)
	}
}
