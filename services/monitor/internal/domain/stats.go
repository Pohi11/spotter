// Package domain holds the drift monitor's statistics and detection rule. Pure functions,
// no I/O (decision record 0012).
package domain

import (
	"math"
	"sort"
)

// MinShare floors every bin share in PSI so empty bins stay finite.
const MinShare = 1e-4

// PSI is the population stability index of window against reference, over 10 bins whose
// edges are the reference's deciles (outer bins open-ended):
//
//	PSI = Σ (w_i − r_i) · ln(w_i / r_i)
func PSI(reference, window []float64) float64 {
	if len(reference) == 0 || len(window) == 0 {
		return 0
	}
	edges := deciles(reference)
	r := shares(reference, edges)
	w := shares(window, edges)
	psi := 0.0
	for i := range r {
		psi += (w[i] - r[i]) * math.Log(w[i]/r[i])
	}
	return psi
}

// deciles returns the 9 inner bin edges at the reference's 10th…90th percentiles.
func deciles(values []float64) []float64 {
	s := sorted(values)
	edges := make([]float64, 9)
	for i := range edges {
		edges[i] = quantile(s, float64(i+1)/10)
	}
	return edges
}

// shares returns the fraction of values in each of the 10 bins, floored at MinShare.
// A value equal to an edge falls in the upper bin.
func shares(values, edges []float64) []float64 {
	counts := make([]float64, len(edges)+1)
	for _, v := range values {
		counts[sort.Search(len(edges), func(i int) bool { return edges[i] > v })]++
	}
	for i := range counts {
		counts[i] = math.Max(counts[i]/float64(len(values)), MinShare)
	}
	return counts
}

// KS returns the two-sample Kolmogorov–Smirnov statistic D (the largest gap between the
// two empirical distribution functions) and its asymptotic p-value.
func KS(reference, window []float64) (d, p float64) {
	a, b := sorted(reference), sorted(window)
	n, m := float64(len(a)), float64(len(b))
	if n == 0 || m == 0 {
		return 0, 1
	}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		// Advance past every value equal to the smaller current value in both samples,
		// so ties don't create artificial gaps.
		x := math.Min(a[i], b[j])
		for i < len(a) && a[i] <= x {
			i++
		}
		for j < len(b) && b[j] <= x {
			j++
		}
		d = math.Max(d, math.Abs(float64(i)/n-float64(j)/m))
	}
	ne := n * m / (n + m)
	return d, kolmogorovQ((math.Sqrt(ne) + 0.12 + 0.11/math.Sqrt(ne)) * d)
}

// kolmogorovQ is the survival function of the Kolmogorov distribution:
// Q(λ) = 2 Σ_{k≥1} (−1)^{k−1} e^{−2k²λ²}.
func kolmogorovQ(lambda float64) float64 {
	if lambda < 1e-3 {
		return 1
	}
	sum := 0.0
	for k := 1; k <= 100; k++ {
		term := math.Exp(-2 * float64(k*k) * lambda * lambda)
		if k%2 == 0 {
			sum -= term
		} else {
			sum += term
		}
		if term < 1e-12 {
			break
		}
	}
	return math.Min(math.Max(2*sum, 0), 1)
}

// Mean of values (0 for none).
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

func sorted(values []float64) []float64 {
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	return s
}

// quantile with linear interpolation between closest ranks; s must be sorted.
func quantile(s []float64, q float64) float64 {
	rank := q * float64(len(s)-1)
	lo, hi := int(math.Floor(rank)), int(math.Ceil(rank))
	return s[lo] + (s[hi]-s[lo])*(rank-float64(lo))
}
