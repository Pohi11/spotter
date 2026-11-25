# Spec Delta

## ADDED Requirements

### Requirement: Each comparison carries the bins behind its PSI
Every feature comparison the monitor publishes (in `DriftEvaluated`, `DriftDetected`, and `DriftResolved`) SHALL include the 9 inner bin edges (the reference's deciles), the reference's share of values in each of the 10 bins, and the window's share in each bin, exactly as used in the PSI, so that summing `(window − reference) · ln(window / reference)` over the bins gives the published PSI.

#### Scenario: Bins reproduce the PSI
- **WHEN** a window is compared with a reference
- **THEN** the comparison has 9 non-decreasing edges and 10 reference and 10 window shares, and the PSI recomputed from the shares equals the reported PSI

#### Scenario: Published with the evaluation
- **WHEN** the monitor publishes a `DriftEvaluated`
- **THEN** each of its features has the edges and both sets of shares
