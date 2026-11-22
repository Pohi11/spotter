package chaos_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Pohi11/spotter/internal/chaos"
	"github.com/Pohi11/spotter/internal/spectest"
)

const capability = "failure-lab"

type recordingFault struct {
	mu             sync.Mutex
	inject, heal   time.Time
	injects, heals int
}

func (f *recordingFault) Inject(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inject, f.injects = time.Now(), f.injects+1
	return nil
}

func (f *recordingFault) Heal(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.heal, f.heals = time.Now(), f.heals+1
	return nil
}

// asyncSystem accepts jobs and completes each after a delay, except while "down".
type asyncSystem struct {
	completions chan chaos.Completion
	n           atomic.Int64
	down        atomic.Bool
}

func (s *asyncSystem) Send(context.Context, []byte) (string, bool, error) {
	if s.down.Load() {
		return "", false, errors.New("unavailable")
	}
	id := fmt.Sprint(s.n.Add(1))
	go func() {
		time.Sleep(5 * time.Millisecond)
		s.completions <- chaos.Completion{JobID: id, At: time.Now()}
	}()
	return id, false, nil
}

type downFault struct{ s *asyncSystem }

func (f downFault) Inject(context.Context) error { f.s.down.Store(true); return nil }
func (f downFault) Heal(context.Context) error   { f.s.down.Store(false); return nil }

var cfg = chaos.Config{Concurrency: 2, Duration: 600 * time.Millisecond, FaultAt: 200 * time.Millisecond,
	HealAt: 400 * time.Millisecond, JobWait: time.Second, Drain: time.Second, Images: [][]byte{[]byte("x")}}

func TestFaultAndHealRunOnSchedule(t *testing.T) {
	spectest.Verifies(t, capability, "A run injects one fault on a schedule and heals it")
	s := &asyncSystem{completions: make(chan chaos.Completion, 1000)}
	f := &recordingFault{}
	start := time.Now()
	_, ev := chaos.Run(context.Background(), cfg, s, s.completions, f)
	if f.injects != 1 || f.heals != 1 {
		t.Fatalf("inject %d, heal %d; want one each", f.injects, f.heals)
	}
	near := func(got time.Time, want time.Duration) bool {
		d := got.Sub(start) - want
		return d > -20*time.Millisecond && d < 100*time.Millisecond
	}
	if !near(f.inject, cfg.FaultAt) || !near(f.heal, cfg.HealAt) {
		t.Fatalf("inject at %v, heal at %v; want about %v and %v", f.inject.Sub(start), f.heal.Sub(start), cfg.FaultAt, cfg.HealAt)
	}
	if ev.InjectErr != "" || ev.HealErr != "" {
		t.Fatalf("events %+v", ev)
	}
}

func TestRejectionsDuringTheFaultAreCounted(t *testing.T) {
	spectest.Verifies(t, capability, "Each run reports what clients and the system experienced")
	s := &asyncSystem{completions: make(chan chaos.Completion, 10000)}
	records, _ := chaos.Run(context.Background(), cfg, s, s.completions, downFault{s})
	rep := chaos.Summarize(records, cfg.FaultAt, cfg.HealAt, cfg.Duration+cfg.Drain)
	if rep.Rejected == 0 || rep.Completed == 0 || rep.Lost != 0 || rep.Duplicates != 0 {
		t.Fatalf("report %+v; want rejections during the fault, completions otherwise, nothing lost or doubled", rep)
	}
	if rep.Requests != rep.Rejected+rep.Completed+rep.Lost {
		t.Fatalf("requests %d != rejected+completed+lost", rep.Requests)
	}
}

func TestLostAndDuplicateJobs(t *testing.T) {
	spectest.Verifies(t, capability, "Each run reports what clients and the system experienced")
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	records := []chaos.Record{
		{Sent: ms(0), JobID: "a", Completed: ms(50), Completions: 1},
		{Sent: ms(10), JobID: "b", Completed: ms(70), Completions: 2},
		{Sent: ms(20), JobID: "c"}, // never completed
		{Sent: ms(30), JobID: "d", Completed: ms(130), Completions: 1},
	}
	rep := chaos.Summarize(records, time.Second, 2*time.Second, 3*time.Second)
	if rep.Completed != 3 || rep.Lost != 1 || rep.Duplicates != 1 || rep.Requests != 4 {
		t.Fatalf("report %+v; want 3 completed, 1 lost, 1 duplicate", rep)
	}
	if rep.P50ms != 60 || rep.MaxMs != 100 {
		t.Fatalf("p50 %v max %v; want 60 and 100", rep.P50ms, rep.MaxMs)
	}
}

// steady returns records completing perSecond jobs in each second of [from, to).
func steady(perSecond func(sec int) int, secs int) []chaos.Record {
	var out []chaos.Record
	for s := range secs {
		for i := range perSecond(s) {
			at := time.Duration(s)*time.Second + time.Duration(i)*time.Millisecond
			out = append(out, chaos.Record{Sent: at, JobID: fmt.Sprint(s, "-", i), Completed: at, Completions: 1})
		}
	}
	return out
}

func TestRecoveryTime(t *testing.T) {
	spectest.Verifies(t, capability, "Each run reports what clients and the system experienced")
	// 50/s before the fault at 10 s; nothing until the third second after heal at 20 s.
	rate := func(s int) int {
		switch {
		case s < 10:
			return 50
		case s < 22:
			return 0
		default:
			return 45
		}
	}
	rep := chaos.Summarize(steady(rate, 30), 10*time.Second, 20*time.Second, 30*time.Second)
	if rep.BaselinePerS != 50 || rep.RecoveryS == nil || *rep.RecoveryS != 3 {
		t.Fatalf("baseline %v recovery %v; want 50 and 3 s", rep.BaselinePerS, rep.RecoveryS)
	}
}

func TestNeverRecovers(t *testing.T) {
	spectest.Verifies(t, capability, "Each run reports what clients and the system experienced")
	rate := func(s int) int {
		if s < 10 {
			return 50
		}
		return 10
	}
	rep := chaos.Summarize(steady(rate, 30), 10*time.Second, 20*time.Second, 30*time.Second)
	if rep.RecoveryS != nil {
		t.Fatalf("recovery %v; want none", *rep.RecoveryS)
	}
}
