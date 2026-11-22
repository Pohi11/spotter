# 0003 — Object store: SeaweedFS behind an S3 port

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Spotter stores two kinds of binary data outside Postgres: uploaded images for inference jobs, and model artifacts (PyTorch and ONNX files, hundreds of megabytes each). Both need an object store that:

- speaks the S3 API, the de facto standard for object storage, so the same code works against any S3-compatible server or a cloud provider;
- runs on a laptop in Docker and inside a Kubernetes cluster;
- is actively maintained and has published container images;
- has a license that allows running it unmodified as part of a demo.

The usual default for self-hosted S3 has been MinIO. Its status was checked on 2026-11-22 against the GitHub API:

| Project | Repository state | License | Latest release | Language |
|---|---|---|---|---|
| MinIO | **archived** (last push 2026-04-24) | AGPL-3.0 | 2025-10-15 | Go |
| SeaweedFS | active (pushed 2026-11-22) | Apache-2.0 | 4.54, 2026-11-16 | Go |
| RustFS | active (pushed 2026-11-22) | Apache-2.0 | 1.0.6, 2026-11-15 | Rust |
| Garage | active (pushed 2026-11-22) | AGPL-3.0 | v2.5.0 (tag) | Rust |

MinIO's community edition stopped publishing binaries and images in October 2025 and its repository is archived, so it gets no security fixes.

## Options considered

1. **MinIO (last community release, or a third-party fork).** Familiar and widely documented, but unmaintained upstream. A fork adds a dependency on a small maintainer group.
2. **SeaweedFS.** Apache-2.0, written in Go, frequent releases, an official Helm chart, and a single-process mode (`weed server -s3`) for local use. More components than the others (master, volume, filer, S3 gateway) when run in its distributed form, and S3 compatibility covers the common operations rather than every feature.
3. **RustFS.** Apache-2.0, aims to be a MinIO replacement with a similar experience. Reached 1.0 recently, so it has the shortest production track record.
4. **Garage.** Simple and light, designed for small geo-distributed clusters. AGPL-3.0 (no obligation when run unmodified), and it implements a deliberately smaller subset of the S3 API.
5. **A cloud provider's object store only.** Breaks the requirement that everything runs on a laptop and in a disconnected cluster.

## Decision

Option 2, SeaweedFS, accessed only through an `ObjectStore` port.

- **The port** exposes only what Spotter needs: put, get, head, delete, and presigned get. Domain and application code never see an S3 type.
- **The S3 adapters** use the standard S3 clients (`aws-sdk-go-v2` in Go, `boto3` in Python) with a configurable endpoint. These are generic S3 protocol clients here; they never touch a cloud account, and they stay inside adapter packages.
- **Buckets** are created by code at startup or by a deployment job, never by hand.
- **The adapter contract tests** run against a real SeaweedFS container. Running the same suite against a second server (RustFS) is the check that the adapter depends on the S3 API rather than on SeaweedFS.

## Consequences

- **Good:** Maintained, permissively licensed, and written in the same language as the control plane.
- **Good:** Swapping the server (RustFS, Garage, a cloud provider) is a configuration change, proven by the contract tests rather than asserted.
- **Cost:** SeaweedFS's distributed mode has more moving parts than a single-binary server. Local and CI setups use single-process mode; the Helm deployment has to choose between its chart and a simpler single-pod setup.
- **Cost:** Only the common S3 operations are guaranteed across servers. Features like object versioning or lifecycle rules can't be relied on without re-evaluating this record.
- **Revisit when:** RustFS has a longer track record, or Spotter needs an S3 feature SeaweedFS doesn't support.
