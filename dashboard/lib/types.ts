// Shapes of the gateway's JSON (protobuf JSON: lowerCamelCase fields, enum names as
// strings, int64 as strings). Message definitions live in proto/spotter/v1/.

export type Stage =
  | "MODEL_VERSION_STAGE_REGISTERED"
  | "MODEL_VERSION_STAGE_VALIDATED"
  | "MODEL_VERSION_STAGE_SHADOW"
  | "MODEL_VERSION_STAGE_PRODUCTION"
  | "MODEL_VERSION_STAGE_RETIRED"
  | "MODEL_VERSION_STAGE_REJECTED"
  | "MODEL_VERSION_STAGE_ROLLED_BACK";

export interface Model {
  id: string;
  name: string;
  description: string;
  createdAt: string;
}

export interface ModelVersion {
  id: string;
  modelId: string;
  version: number;
  artifactUri: string;
  artifactSha256: string;
  framework: string;
  metrics: Record<string, number>;
  stage: Stage;
  revision: string;
  createdAt: string;
  updatedAt: string;
}

export interface StageChange {
  fromStage: Stage;
  toStage: Stage;
  reason: string;
  revision: string;
  changedAt: string;
}

export interface Gate {
  name: string;
  passed: boolean;
  detail: string;
}

export interface Gates {
  gates: Gate[];
  allPassed: boolean;
}

export interface ClassScore {
  label: string;
  score: number;
}

export interface Prediction {
  label: string;
  confidence: number;
  scores: ClassScore[];
  modelVersionId: string;
  modelVersion: number;
}

export type JobStatus = "JOB_STATUS_PENDING" | "JOB_STATUS_SUCCEEDED" | "JOB_STATUS_FAILED";

export interface Job {
  jobId: string;
  modelName: string;
  status: JobStatus;
  prediction: Prediction | null;
  error: string;
  imageUri: string;
  submittedAt: string;
  completedAt: string | null;
}

/** One event from GET /v1/events (spec: event-feed). */
export interface FeedEvent {
  stream: "INFERENCE" | "REGISTRY" | "MONITOR" | string;
  subject: string;
  sequence: number;
  time: string;
  type: string;
  payload: Record<string, unknown> | null;
}

export interface FeatureDrift {
  feature: string;
  psi: number;
  ksStatistic: number;
  ksPValue: number;
  referenceMean: number;
  windowMean: number;
}

export interface DriftEvaluated {
  modelName: string;
  versionId: string;
  version: number;
  features: FeatureDrift[];
  windowSize: number;
  drifting: boolean;
  evaluatedAt: string;
}

/** A sample tile and its degradations (public/samples/manifest.json). */
export interface Sample {
  id: string;
  true_class: string;
  source: string;
  variants: { degradation: string; strength: number; file: string }[];
}
