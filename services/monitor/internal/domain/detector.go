package domain

// Feature is one per-image quantity the monitor watches.
type Feature string

const (
	MeanR      Feature = "mean_r"
	MeanG      Feature = "mean_g"
	MeanB      Feature = "mean_b"
	Contrast   Feature = "contrast"
	Confidence Feature = "confidence"
)

// Features lists every watched feature, in a fixed order.
var Features = []Feature{MeanR, MeanG, MeanB, Contrast, Confidence}

// Sample is one inference's features, indexed like Features.
type Sample [5]float64

// Reference is a version's training-time distribution of each feature.
type Reference struct {
	VersionID string
	Values    map[Feature][]float64
}

// Config is the detection rule (design D3 of the drift monitor change).
type Config struct {
	Window int     // events per comparison
	Every  int     // evaluate after this many new events
	Enter  float64 // PSI at or above which a feature is drifting
	Exit   float64 // all PSI below this to stop drifting
}

// DefaultConfig: window 200, every 50 events, enter at 0.2, leave below 0.1.
var DefaultConfig = Config{Window: 200, Every: 50, Enter: 0.2, Exit: 0.1}

// Result is one feature's comparison of the window with the reference.
type Result struct {
	Feature       Feature
	PSI           float64
	KSStatistic   float64
	KSPValue      float64
	ReferenceMean float64
	WindowMean    float64
}

// Transition is what an evaluation changed.
type Transition int

const (
	NoChange Transition = iota
	Detected
	Resolved
)

// Detector tracks one model version: a sliding window and a drifting/not-drifting state.
type Detector struct {
	cfg       Config
	ref       Reference
	refMeans  [5]float64
	window    []Sample // ring buffer
	next      int
	full      bool
	sinceEval int
	drifting  bool
}

// NewDetector starts a detector for a version with its reference.
func NewDetector(ref Reference, cfg Config) *Detector {
	d := &Detector{cfg: cfg, ref: ref, window: make([]Sample, cfg.Window)}
	for i, f := range Features {
		d.refMeans[i] = Mean(ref.Values[f])
	}
	return d
}

// Drifting reports the current state.
func (d *Detector) Drifting() bool { return d.drifting }

// Observe adds one sample. Every cfg.Every samples once the window is full it compares
// each feature with the reference and returns the results and any state change.
// Results are nil when no evaluation ran.
func (d *Detector) Observe(s Sample) (Transition, []Result) {
	d.window[d.next] = s
	d.next = (d.next + 1) % len(d.window)
	if d.next == 0 {
		d.full = true
	}
	d.sinceEval++
	if !d.full || d.sinceEval < d.cfg.Every {
		return NoChange, nil
	}
	d.sinceEval = 0

	results := make([]Result, len(Features))
	maxPSI := 0.0
	for i, f := range Features {
		values := make([]float64, len(d.window))
		for j, w := range d.window {
			values[j] = w[i]
		}
		ksD, ksP := KS(d.ref.Values[f], values)
		results[i] = Result{Feature: f, PSI: PSI(d.ref.Values[f], values), KSStatistic: ksD, KSPValue: ksP,
			ReferenceMean: d.refMeans[i], WindowMean: Mean(values)}
		maxPSI = max(maxPSI, results[i].PSI)
	}
	switch {
	case !d.drifting && maxPSI >= d.cfg.Enter:
		d.drifting = true
		return Detected, results
	case d.drifting && maxPSI < d.cfg.Exit:
		d.drifting = false
		return Resolved, results
	default:
		return NoChange, results
	}
}

// Drifted returns the results at or above the enter threshold.
func (c Config) Drifted(results []Result) []Result {
	var out []Result
	for _, r := range results {
		if r.PSI >= c.Enter {
			out = append(out, r)
		}
	}
	return out
}
