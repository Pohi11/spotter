package domain_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/monitor/internal/domain"
)

const capability = "drift-monitoring"

func normal(r *rand.Rand, n int, mean, sd float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = r.NormFloat64()*sd + mean
	}
	return out
}

func TestPSISameDistribution(t *testing.T) {
	spectest.Verifies(t, capability, "PSI compares a window to the reference")
	// Sampling noise alone: E[PSI] ≈ (bins − 1)(1/n + 1/N) = 9 × (1/200 + 1/4000) ≈ 0.047.
	r := rand.New(rand.NewPCG(1, 2))
	ref := normal(r, 4000, 0.4, 0.1)
	sum := 0.0
	for range 20 {
		sum += domain.PSI(ref, normal(r, 200, 0.4, 0.1))
	}
	if mean := sum / 20; mean >= 0.1 {
		t.Fatalf("mean PSI of same-distribution windows = %.3f, want < 0.1", mean)
	} else {
		t.Logf("mean PSI over 20 same-distribution windows: %.3f", mean)
	}
}

func TestPSIShiftedDistribution(t *testing.T) {
	spectest.Verifies(t, capability, "PSI compares a window to the reference")
	r := rand.New(rand.NewPCG(1, 2))
	ref := normal(r, 4000, 0.4, 0.1)
	win := make([]float64, 200)
	for i := range win {
		win[i] = ref[i] + 3*0.1
	}
	if psi := domain.PSI(ref, win); psi <= 1.0 {
		t.Fatalf("PSI of 3-sd shift = %.3f, want > 1.0", psi)
	}
}

func TestPSIEmptyBinsStayFinite(t *testing.T) {
	spectest.Verifies(t, capability, "PSI compares a window to the reference")
	r := rand.New(rand.NewPCG(1, 2))
	win := make([]float64, 200)
	for i := range win {
		win[i] = 10 // all in the top bin
	}
	if psi := domain.PSI(normal(r, 4000, 0.4, 0.1), win); math.IsInf(psi, 0) || math.IsNaN(psi) {
		t.Fatalf("PSI = %v, want finite", psi)
	}
}

func TestBinsReproduceThePSI(t *testing.T) {
	spectest.Verifies(t, capability, "Each comparison carries the bins behind its PSI")
	r := rand.New(rand.NewPCG(3, 4))
	ref := normal(r, 4000, 0.4, 0.1)
	win := normal(r, 200, 0.5, 0.1)
	b := domain.Compare(ref, win)
	if len(b.Edges) != 9 || len(b.Reference) != 10 || len(b.Window) != 10 {
		t.Fatalf("got %d edges, %d reference and %d window shares", len(b.Edges), len(b.Reference), len(b.Window))
	}
	for i := 1; i < len(b.Edges); i++ {
		if b.Edges[i] < b.Edges[i-1] {
			t.Fatalf("edges decrease at %d: %v", i, b.Edges)
		}
	}
	sum := 0.0
	for i := range b.Reference {
		sum += (b.Window[i] - b.Reference[i]) * math.Log(b.Window[i]/b.Reference[i])
	}
	if psi := domain.PSI(ref, win); math.Abs(sum-psi) > 1e-12 || psi < 0.2 {
		t.Fatalf("recomputed %v, PSI %v (want equal, and a shift of 1 sd to drift)", sum, psi)
	}
	// Deciles of the reference put about a tenth of it in each bin.
	for i, s := range b.Reference {
		if math.Abs(s-0.1) > 0.01 {
			t.Fatalf("reference share in bin %d = %v, want about 0.1", i, s)
		}
	}
}

func TestDetectorResultsCarryBins(t *testing.T) {
	spectest.Verifies(t, capability, "Each comparison carries the bins behind its PSI")
	r := rand.New(rand.NewPCG(5, 6))
	values := map[domain.Feature][]float64{}
	for _, f := range domain.Features {
		values[f] = normal(r, 1000, 0.4, 0.1)
	}
	d := domain.NewDetector(domain.Reference{VersionID: "v", Values: values}, domain.DefaultConfig)
	var results []domain.Result
	for range 200 {
		var s domain.Sample
		for i := range s {
			s[i] = r.NormFloat64()*0.1 + 0.4
		}
		_, results = d.Observe(s)
	}
	if len(results) != len(domain.Features) {
		t.Fatalf("no evaluation after a full window")
	}
	for _, res := range results {
		if len(res.Bins.Window) != 10 || math.Abs(res.Bins.PSI()-res.PSI) > 1e-12 {
			t.Fatalf("%s: bins %v give PSI %v, result says %v", res.Feature, res.Bins, res.Bins.PSI(), res.PSI)
		}
	}
}

func TestKSKnownStatistic(t *testing.T) {
	spectest.Verifies(t, capability, "The KS test is reported with each comparison")
	d, p := domain.KS([]float64{1, 2, 3, 4}, []float64{3, 4, 5, 6})
	if d != 0.5 {
		t.Fatalf("D = %v, want 0.5", d)
	}
	if p <= 0 || p > 1 {
		t.Fatalf("p = %v, want in (0, 1]", p)
	}
}

func TestKSIdenticalSamples(t *testing.T) {
	spectest.Verifies(t, capability, "The KS test is reported with each comparison")
	s := []float64{0.1, 0.2, 0.2, 0.5, 0.9}
	if d, p := domain.KS(s, s); d != 0 || p != 1 {
		t.Fatalf("D = %v, p = %v; want 0 and 1", d, p)
	}
}

// reference and sample generators for detector tests: brightness features ~0.4, contrast
// ~0.12, confidence ~0.95.
func reference(r *rand.Rand) domain.Reference {
	return domain.Reference{VersionID: "v1", Values: map[domain.Feature][]float64{
		domain.MeanR: normal(r, 4000, 0.34, 0.08), domain.MeanG: normal(r, 4000, 0.38, 0.07),
		domain.MeanB: normal(r, 4000, 0.41, 0.06), domain.Contrast: normal(r, 4000, 0.12, 0.03),
		domain.Confidence: normal(r, 4000, 0.95, 0.03),
	}}
}

func sample(r *rand.Rand, brightness float64) domain.Sample {
	return domain.Sample{
		(r.NormFloat64()*0.08 + 0.34) * brightness, (r.NormFloat64()*0.07 + 0.38) * brightness,
		(r.NormFloat64()*0.06 + 0.41) * brightness, r.NormFloat64()*0.03 + 0.12, r.NormFloat64()*0.03 + 0.95,
	}
}

// feed sends n samples and returns every transition with the results that came with it.
func feed(d *domain.Detector, r *rand.Rand, n int, brightness float64) (transitions []domain.Transition, last []domain.Result) {
	for range n {
		tr, res := d.Observe(sample(r, brightness))
		if tr != domain.NoChange {
			transitions = append(transitions, tr)
			last = res
		}
	}
	return transitions, last
}

func TestCleanTrafficRaisesNothing(t *testing.T) {
	spectest.Verifies(t, capability, "Drift is announced once per change of state")
	r := rand.New(rand.NewPCG(3, 4))
	d := domain.NewDetector(reference(r), domain.DefaultConfig)
	if trs, _ := feed(d, r, 2000, 1.0); len(trs) != 0 {
		t.Fatalf("clean traffic produced transitions %v", trs)
	}
}

func TestDarkenedImagesAreDetectedOnce(t *testing.T) {
	spectest.Verifies(t, capability, "Drift is announced once per change of state")
	spectest.Verifies(t, capability, "Drift events describe the shifted features")
	r := rand.New(rand.NewPCG(3, 4))
	d := domain.NewDetector(reference(r), domain.DefaultConfig)
	feed(d, r, 400, 1.0)
	trs, results := feed(d, r, 1000, 0.5) // half brightness, for many evaluations
	if len(trs) != 1 || trs[0] != domain.Detected {
		t.Fatalf("transitions = %v, want exactly one Detected", trs)
	}
	drifted := domain.DefaultConfig.Drifted(results)
	names := map[domain.Feature]bool{}
	for _, res := range drifted {
		names[res.Feature] = true
		if res.WindowMean >= res.ReferenceMean {
			t.Errorf("%s window mean %.3f not below reference %.3f", res.Feature, res.WindowMean, res.ReferenceMean)
		}
	}
	if !names[domain.MeanR] || !names[domain.MeanG] || !names[domain.MeanB] {
		t.Fatalf("drifted features = %v, want the brightness features", names)
	}
}

func TestReturnToNormalResolvesOnce(t *testing.T) {
	spectest.Verifies(t, capability, "Drift is announced once per change of state")
	r := rand.New(rand.NewPCG(3, 4))
	d := domain.NewDetector(reference(r), domain.DefaultConfig)
	feed(d, r, 400, 0.5)
	if !d.Drifting() {
		t.Fatal("not drifting after darkened traffic")
	}
	trs, _ := feed(d, r, 600, 1.0)
	if len(trs) != 1 || trs[0] != domain.Resolved {
		t.Fatalf("transitions = %v, want exactly one Resolved", trs)
	}
}
