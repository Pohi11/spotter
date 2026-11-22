# Local development on Kubernetes (decision record 0014).
#
#   k3d cluster create --config deploy/k3d/cluster.yaml
#   tilt up
#
# Builds the images, installs KEDA and the Spotter chart, and port-forwards the gateway
# (REST 8080, gRPC 9091), the dashboard (3000), and NATS (4222). Source edits rebuild the affected image and roll
# the workloads that use it.

load('ext://helm_resource', 'helm_resource', 'helm_repo')

allow_k8s_contexts('k3d-spotter')

# Chart installs wait for pods (KEDA, Postgres, image pulls), well past the 30 s default.
update_settings(k8s_upsert_timeout_secs=900)
# Don't prune Docker build caches: the Python image's dependency layer is expensive to rebuild,
# and the caches may belong to other projects.
docker_prune_settings(disable=True)

docker_build(
    'spotter-go', '.',
    dockerfile='deploy/docker/go.Dockerfile',
    only=['go.mod', 'go.sum', 'cmd', 'gen', 'internal', 'services', 'tools'],
)
docker_build(
    'spotter-python', '.',
    dockerfile='deploy/docker/python.Dockerfile',
    only=['pyproject.toml', 'uv.lock', 'gen/python', 'workers'],
    ignore=['**/__pycache__', 'workers/*/tests'],
)

# The dashboard reads the dated benchmark reports at build time; copy them into its context.
local('node dashboard/scripts/sync-content.mjs')
docker_build(
    'spotter-dashboard', 'dashboard',
    dockerfile='deploy/docker/dashboard.Dockerfile',
    ignore=['node_modules', '.next', '.next-e2e', 'test-results', 'e2e'],
)

helm_repo('kedacore', 'https://kedacore.github.io/charts', labels=['infra'])
helm_resource(
    'keda', 'kedacore/keda',
    namespace='keda',
    flags=['--create-namespace', '--version', '2.21.1', '--wait'],
    resource_deps=['kedacore'],
    labels=['infra'],
)

# A real `helm upgrade --install` (not `helm template`), so the chart's generated Secret is
# kept across reloads instead of regenerated.
helm_resource(
    'spotter', 'deploy/helm/spotter',
    namespace='spotter',
    # --wait: the resource is ready only when every workload is.
    flags=['--create-namespace', '--values', 'deploy/helm/values-dev.yaml', '--wait', '--timeout', '10m'],
    image_deps=['spotter-go', 'spotter-python', 'spotter-dashboard'],
    image_keys=[
        ('images.go.repository', 'images.go.tag'),
        ('images.python.repository', 'images.python.tag'),
        ('images.dashboard.repository', 'images.dashboard.tag'),
    ],
    resource_deps=['keda'],
    labels=['spotter'],
)

local_resource(
    'port-forwards',
    serve_cmd='kubectl -n spotter port-forward svc/spotter-gateway 8080:8080 9091:9091',
    resource_deps=['spotter'],
    labels=['spotter'],
)
local_resource(
    'dashboard-port-forward',
    serve_cmd='kubectl -n spotter port-forward svc/spotter-dashboard 3000:3000',
    resource_deps=['spotter'],
    labels=['spotter'],
)
local_resource(
    'nats-port-forward',
    serve_cmd='kubectl -n spotter port-forward svc/spotter-nats 4222:4222',
    resource_deps=['spotter'],
    labels=['spotter'],
)
