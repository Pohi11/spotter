# Spec Delta

## Purpose

Defines the metrics reported for a trained classifier and how the holdout set is protected.

## ADDED Requirements

### Requirement: Evaluation reports per-class metrics
Evaluating predictions against true labels SHALL report the confusion matrix (rows are true classes, columns are predicted classes), overall accuracy, precision and recall for each class, and macro F1 (the unweighted mean of per-class F1). A class that is never predicted SHALL have precision 0, and a class with no true examples SHALL have recall 0, rather than an undefined value.

#### Scenario: Known predictions
- **WHEN** true labels `[0, 0, 1, 1]` are evaluated against predictions `[0, 1, 1, 1]` for 2 classes
- **THEN** the confusion matrix is `[[1, 1], [0, 2]]`, accuracy is 0.75, class 0 has precision 1.0 and recall 0.5, and class 1 has precision 2/3 and recall 1.0

#### Scenario: A class is never predicted
- **WHEN** true labels `[0, 1, 2]` are evaluated against predictions `[0, 0, 0]` for 3 classes
- **THEN** classes 1 and 2 have precision 0 and recall 0, and no metric is NaN

#### Scenario: Perfect predictions
- **WHEN** predictions equal the true labels
- **THEN** accuracy, every precision, every recall, and macro F1 are all 1.0

### Requirement: The holdout set is used only for final evaluation
Training and model selection SHALL read only the training and validation sets. The holdout set SHALL be read once, after training finishes, to compute the metrics that are published and registered.

#### Scenario: Holdout untouched during training
- **WHEN** a model is trained and evaluated
- **THEN** no holdout image is read before the last training epoch completes
- **AND** the published metrics come from the holdout set
