# platform-deployment Specification

## Purpose
Defines how the platform is packaged for Kubernetes and what an install guarantees.

## Requirements

### Requirement: One chart declares the platform
The Helm chart SHALL render, with default values, Deployments for the registry, gateway, monitor, and inference worker; StatefulSets for Postgres and SeaweedFS; and NATS with JetStream from the official NATS chart. With `postgres.enabled=false`, it SHALL render no Postgres and SHALL take the database URL from an existing Secret.

#### Scenario: Default render
- **WHEN** the chart is rendered with default values
- **THEN** it contains the four Deployments, the two StatefulSets, and a NATS StatefulSet with JetStream enabled

#### Scenario: External database
- **WHEN** the chart is rendered with `postgres.enabled=false` and `externalDatabase.existingSecret=db`
- **THEN** it contains no Postgres StatefulSet and the registry reads `DATABASE_URL` from Secret `db`

### Requirement: Credentials come from a generated Secret
The database password and the S3 access and secret keys SHALL reach workloads only through Secret references. The chart SHALL generate random values at first install, keep the existing values on upgrade, and accept an existing Secret instead. No credential SHALL appear in a ConfigMap or as a literal environment value.

#### Scenario: Default render
- **WHEN** the chart is rendered with default values
- **THEN** every credential environment variable uses a `secretKeyRef` and no ConfigMap contains a credential

#### Scenario: Upgrade keeps credentials
- **WHEN** an installed release is upgraded
- **THEN** the generated Secret's values are unchanged

### Requirement: Every workload reports health and declares resources
Every container of the chart's Deployments and StatefulSets SHALL have a readiness probe, a liveness probe, and CPU and memory requests and limits. The Go services' probes SHALL use `/readyz` and `/livez`.

#### Scenario: Probes and resources
- **WHEN** the chart is rendered with default values
- **THEN** every container has both probes and all four resource values, and the Go services probe `/readyz` and `/livez`

### Requirement: Pods stop gracefully
Each Go service's termination grace period SHALL exceed its `SHUTDOWN_TIMEOUT`. The worker's `MODEL_THREADS` SHALL equal its CPU limit in whole cores (at least 1), so its model threads never exceed the CPU it may use.

#### Scenario: Grace periods and threads
- **WHEN** the chart is rendered with a worker CPU limit of 2
- **THEN** each Go service's grace period is longer than its shutdown timeout and the worker's `MODEL_THREADS` is 2

### Requirement: Workers scale on consumer lag
With `worker.autoscaling.enabled=true`, the chart SHALL render a KEDA ScaledObject that scales the worker Deployment between the configured minimum and maximum replicas on the lag of consumer `inference-worker-<model>` on stream `INFERENCE`, and the worker Deployment SHALL leave its replica count to KEDA.

#### Scenario: Autoscaling on
- **WHEN** the chart is rendered with autoscaling enabled, minimum 1, maximum 4, and a lag threshold of 20
- **THEN** a ScaledObject targets the worker Deployment with those bounds and a `nats-jetstream` trigger for that stream and consumer, and the Deployment sets no replica count

#### Scenario: Autoscaling off
- **WHEN** the chart is rendered with autoscaling disabled and 2 worker replicas
- **THEN** there is no ScaledObject and the worker Deployment has 2 replicas

### Requirement: A trainer Job is available
With `trainer.enabled=true`, the chart SHALL render a Job that runs the trainer from the Python image with the registry address, S3 settings, and credentials from the Secret, and that is not restarted on failure.

#### Scenario: Trainer enabled
- **WHEN** the chart is rendered with `trainer.enabled=true`
- **THEN** a Job runs `spotter-train` with restart policy `Never` and reads its S3 keys from the Secret

### Requirement: The platform installs into an empty cluster
Installing the chart into an empty cluster (with KEDA when autoscaling is enabled) SHALL bring every workload to ready, and `helm test` SHALL pass a smoke test: the gateway and registry report ready, a model registered through the gateway's REST API can be read back, and the worker answers a NATS ping.

#### Scenario: Fresh install
- **WHEN** the chart is installed into an empty cluster and `helm test` runs
- **THEN** every Deployment and StatefulSet is ready and the smoke test passes
