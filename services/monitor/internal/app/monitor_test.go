package app_test

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/monitor/internal/app"
	"github.com/Pohi11/spotter/services/monitor/internal/domain"
	"github.com/Pohi11/spotter/services/monitor/internal/ports"
)

const capability = "drift-monitoring"

type fakes struct {
	mu        sync.Mutex
	refs      map[string]domain.Reference
	published []ports.DriftEvent
	skipped   map[string]int
	psi       map[string]float64
}

func (f *fakes) Reference(_ context.Context, id string) (domain.Reference, error) {
	if r, ok := f.refs[id]; ok {
		return r, nil
	}
	return domain.Reference{}, ports.ErrNoReference
}
func (f *fakes) Publish(_ context.Context, e ports.DriftEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, e)
	return nil
}
func (f *fakes) PSI(_, v string, feat domain.Feature, psi float64) { f.psi[v+"/"+string(feat)] = psi }
func (f *fakes) Skipped(_, v string)                               { f.skipped[v]++ }
func (f *fakes) Now() time.Time                                    { return time.Unix(0, 0) }

func ref(r *rand.Rand, id string, brightness float64) domain.Reference {
	vals := map[domain.Feature][]float64{}
	for i, f := range domain.Features {
		v := make([]float64, 4000)
		for j := range v {
			v[j] = r.NormFloat64()*0.05 + 0.4*brightness + float64(i)*0.01
		}
		vals[f] = v
	}
	return domain.Reference{VersionID: id, Values: vals}
}

func obs(r *rand.Rand, version string, brightness float64, seq uint64) app.Observation {
	var s domain.Sample
	for i := range s {
		s[i] = r.NormFloat64()*0.05 + 0.4*brightness + float64(i)*0.01
	}
	return app.Observation{Model: "eurosat-classifier", VersionID: version, Version: 1, Sample: s, Sequence: seq}
}

func newMonitor(f *fakes) *app.Monitor {
	return app.NewMonitor(f, f, f, f, domain.DefaultConfig)
}

func TestTwoVersionsAreTrackedSeparately(t *testing.T) {
	spectest.Verifies(t, capability, "Each version is compared with its own reference")
	r := rand.New(rand.NewPCG(5, 6))
	// v1 was trained on bright images, v2 on dark ones; each sees traffic like its own.
	f := &fakes{refs: map[string]domain.Reference{"v1": ref(r, "v1", 1.0), "v2": ref(r, "v2", 0.5)},
		skipped: map[string]int{}, psi: map[string]float64{}}
	m := newMonitor(f)
	for i := range 600 {
		if err := m.Observe(context.Background(), obs(r, "v1", 1.0, uint64(2*i))); err != nil {
			t.Fatal(err)
		}
		if err := m.Observe(context.Background(), obs(r, "v2", 0.5, uint64(2*i+1))); err != nil {
			t.Fatal(err)
		}
	}
	// Mixing the windows would make both look drifted against their references.
	if len(f.published) != 0 {
		t.Fatalf("published %d drift events; windows were mixed", len(f.published))
	}
	if _, ok := f.psi["v1/mean_r"]; !ok {
		t.Fatal("no PSI recorded for v1")
	}
}

func TestMissingReferenceIsSkippedAndCounted(t *testing.T) {
	spectest.Verifies(t, capability, "Each version is compared with its own reference")
	r := rand.New(rand.NewPCG(5, 6))
	f := &fakes{refs: map[string]domain.Reference{"v1": ref(r, "v1", 1.0)}, skipped: map[string]int{}, psi: map[string]float64{}}
	m := newMonitor(f)
	for i := range 300 {
		if err := m.Observe(context.Background(), obs(r, "v-unknown", 0.2, uint64(i))); err != nil {
			t.Fatal(err)
		}
	}
	if f.skipped["v-unknown"] != 300 || len(f.published) != 0 {
		t.Fatalf("skipped = %d, published = %d; want 300 and 0", f.skipped["v-unknown"], len(f.published))
	}
	if _, tracked := m.Drifting("v-unknown"); tracked {
		t.Fatal("a version without a reference got a detector")
	}
}

func TestDetectedEventNamesOnlyDriftedFeatures(t *testing.T) {
	spectest.Verifies(t, capability, "Drift events describe the shifted features")
	r := rand.New(rand.NewPCG(5, 6))
	f := &fakes{refs: map[string]domain.Reference{"v1": ref(r, "v1", 1.0)}, skipped: map[string]int{}, psi: map[string]float64{}}
	m := newMonitor(f)
	for i := range 400 {
		if err := m.Observe(context.Background(), obs(r, "v1", 0.5, uint64(i))); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.published) != 1 || f.published[0].Transition != domain.Detected {
		t.Fatalf("published = %+v, want one Detected", f.published)
	}
	for _, res := range f.published[0].Results {
		if res.PSI < 0.2 {
			t.Errorf("feature %s with PSI %.3f listed as drifted", res.Feature, res.PSI)
		}
	}
	if f.published[0].DedupKey == "" {
		t.Error("no de-duplication key")
	}
}
