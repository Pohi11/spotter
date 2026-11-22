# training-data Specification

## Purpose
Defines how the EuroSAT dataset is divided into training, validation, and holdout sets, and what is guaranteed about those sets.

## Requirements

### Requirement: Splits are deterministic
Splitting SHALL depend only on the list of image paths and a seed. The same images and seed SHALL always produce the same three sets, and the split SHALL have a fingerprint (SHA-256 of its manifest) that changes whenever any image moves between sets.

#### Scenario: Same seed, same split
- **WHEN** the same images are split twice with seed 42
- **THEN** both splits are identical and have the same fingerprint

#### Scenario: Different seed, different split
- **WHEN** the same images are split with seed 42 and with seed 7
- **THEN** the splits differ and so do their fingerprints

#### Scenario: Input order does not matter
- **WHEN** the same images are split with seed 42, once in sorted order and once shuffled
- **THEN** both splits are identical

### Requirement: Splits are stratified and disjoint
Each class SHALL be divided 70% / 15% / 15% into training, validation, and holdout sets, rounding so that every image is assigned. No image SHALL appear in more than one set.

#### Scenario: Class proportions
- **WHEN** a class of 3,000 images is split
- **THEN** 2,100 are in training, 450 in validation, and 450 in holdout

#### Scenario: No overlap
- **WHEN** a dataset is split
- **THEN** the three sets share no image and together contain every image exactly once
