// Package helm_test checks the Spotter Helm chart (change phase-09-kubernetes, design D7).
//
// Render tests run `helm template` and inspect the objects; they skip when helm isn't on
// PATH. The install tests act on a real cluster and run only when SPOTTER_KUBE_CONTEXT names
// the kube context to use; they never fall back to the current context.
package helm_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/Pohi11/spotter/internal/spectest"
)

const capability = "platform-deployment"

type object map[string]any

func (o object) kind() string { return str(o, "kind") }
func (o object) name() string { return str(o, "metadata", "name") }

// get walks maps by key and slices by index ("0").
func get(v any, path ...string) any {
	for _, p := range path {
		switch m := v.(type) {
		case object:
			v = map[string]any(m)[p]
		case map[string]any:
			v = m[p]
		case []any:
			i, err := strconv.Atoi(p)
			if err != nil || i >= len(m) {
				return nil
			}
			v = m[i]
		default:
			return nil
		}
	}
	return v
}

func str(v any, path ...string) string {
	s, _ := get(v, path...).(string)
	return s
}

// dict returns the map at path (the YAML decoder gives nested maps the parent's type).
func dict(v any, path ...string) map[string]any {
	switch m := get(v, path...).(type) {
	case object:
		return m
	case map[string]any:
		return m
	}
	return nil
}

func list(v any, path ...string) []any {
	l, _ := get(v, path...).([]any)
	return l
}

// render runs helm template with the given --set values and returns the objects.
func render(t *testing.T, sets ...string) []object {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not on PATH")
	}
	args := []string{"template", "spotter", "spotter", "--namespace", "spotter"}
	for _, s := range sets {
		args = append(args, "--set", s)
	}
	out, err := exec.Command("helm", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("helm template: %v\n%s", err, out)
	}
	var objs []object
	dec := yaml.NewDecoder(bytes.NewReader(out))
	for {
		var o object
		err := dec.Decode(&o)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode rendered YAML: %v", err)
		}
		if o != nil {
			objs = append(objs, o)
		}
	}
	return objs
}

func find(objs []object, kind, name string) object {
	for _, o := range objs {
		if o.kind() == kind && o.name() == name {
			return o
		}
	}
	return nil
}

func ofKind(objs []object, kinds ...string) []object {
	var out []object
	for _, o := range objs {
		for _, k := range kinds {
			if o.kind() == k {
				out = append(out, o)
			}
		}
	}
	return out
}

// containers returns a workload's containers (not init containers).
func containers(o object) []any {
	if o.kind() == "Pod" {
		return list(o, "spec", "containers")
	}
	if o.kind() == "Job" {
		return list(o, "spec", "template", "spec", "containers")
	}
	return list(o, "spec", "template", "spec", "containers")
}

func envVar(container any, name string) any {
	for _, e := range list(container, "env") {
		if str(e, "name") == name {
			return e
		}
	}
	return nil
}

func TestDefaultRender(t *testing.T) {
	spectest.Verifies(t, capability, "One chart declares the platform")
	objs := render(t)
	for _, name := range []string{"registry", "gateway", "monitor", "worker"} {
		if find(objs, "Deployment", "spotter-"+name) == nil {
			t.Errorf("no Deployment spotter-%s", name)
		}
	}
	for _, name := range []string{"postgres", "seaweedfs", "nats"} {
		if find(objs, "StatefulSet", "spotter-"+name) == nil {
			t.Errorf("no StatefulSet spotter-%s", name)
		}
	}
	natsConfig := find(objs, "ConfigMap", "spotter-nats-config")
	if natsConfig == nil || !strings.Contains(str(natsConfig, "data", "nats.conf"), "jetstream") {
		t.Errorf("NATS config without JetStream: %v", natsConfig)
	}
}

func TestExternalDatabase(t *testing.T) {
	spectest.Verifies(t, capability, "One chart declares the platform")
	objs := render(t, "postgres.enabled=false", "externalDatabase.existingSecret=db")
	if find(objs, "StatefulSet", "spotter-postgres") != nil {
		t.Fatal("Postgres rendered although disabled")
	}
	ref := get(envVar(containers(find(objs, "Deployment", "spotter-registry"))[0], "DATABASE_URL"), "valueFrom", "secretKeyRef")
	if str(ref, "name") != "db" || str(ref, "key") != "database-url" {
		t.Fatalf("DATABASE_URL from %v, want secret db key database-url", ref)
	}
}

var credentialVars = map[string]bool{"DATABASE_URL": true, "POSTGRES_PASSWORD": true, "S3_ACCESS_KEY": true, "S3_SECRET_KEY": true}

func TestCredentialsOnlyFromSecrets(t *testing.T) {
	spectest.Verifies(t, capability, "Credentials come from a generated Secret")
	objs := render(t, "trainer.enabled=true")
	secret := find(objs, "Secret", "spotter-credentials")
	if secret == nil {
		t.Fatal("no generated Secret")
	}
	var values []string
	for key, v := range dict(secret, "data") {
		raw, err := base64.StdEncoding.DecodeString(v.(string))
		if err != nil || len(raw) < 16 {
			t.Fatalf("secret key %s: %d bytes, %v", key, len(raw), err)
		}
		if key != "database-url" {
			values = append(values, string(raw))
		}
	}
	checked := 0
	for _, o := range ofKind(objs, "Deployment", "StatefulSet", "Job", "Pod") {
		for _, c := range containers(o) {
			for _, e := range list(c, "env") {
				name, literal := str(e, "name"), str(e, "value")
				if credentialVars[name] {
					checked++
					if get(e, "valueFrom", "secretKeyRef") == nil {
						t.Errorf("%s/%s: %s is not a secretKeyRef", o.kind(), o.name(), name)
					}
				}
				for _, v := range values {
					if strings.Contains(literal, v) {
						t.Errorf("%s/%s: env %s holds a credential", o.kind(), o.name(), name)
					}
				}
			}
		}
	}
	if checked < 10 {
		t.Fatalf("only %d credential variables found; the check is not looking at the workloads", checked)
	}
	for _, cm := range ofKind(objs, "ConfigMap") {
		for _, data := range dict(cm, "data") {
			for _, v := range values {
				if strings.Contains(data.(string), v) {
					t.Errorf("ConfigMap %s holds a credential", cm.name())
				}
			}
		}
	}
}

func TestProbesAndResources(t *testing.T) {
	spectest.Verifies(t, capability, "Every workload reports health and declares resources")
	objs := render(t)
	workloads := ofKind(objs, "Deployment", "StatefulSet")
	if len(workloads) != 7 {
		t.Fatalf("%d workloads, want 7", len(workloads))
	}
	for _, o := range workloads {
		for _, c := range containers(o) {
			id := o.kind() + "/" + o.name() + "/" + str(c, "name")
			for _, probe := range []string{"livenessProbe", "readinessProbe"} {
				if get(c, probe) == nil {
					t.Errorf("%s: no %s", id, probe)
				}
			}
			for _, r := range []string{"requests", "limits"} {
				for _, q := range []string{"cpu", "memory"} {
					if get(c, "resources", r, q) == nil {
						t.Errorf("%s: no resources.%s.%s", id, r, q)
					}
				}
			}
		}
	}
	for _, name := range []string{"registry", "gateway", "monitor"} {
		c := containers(find(objs, "Deployment", "spotter-"+name))[0]
		if p := str(c, "livenessProbe", "httpGet", "path"); p != "/livez" {
			t.Errorf("%s liveness path %q", name, p)
		}
		if p := str(c, "readinessProbe", "httpGet", "path"); p != "/readyz" {
			t.Errorf("%s readiness path %q", name, p)
		}
	}
}

func TestGracePeriodsAndThreads(t *testing.T) {
	spectest.Verifies(t, capability, "Pods stop gracefully")
	objs := render(t, "worker.resources.limits.cpu=2")
	for _, name := range []string{"registry", "gateway", "monitor"} {
		d := find(objs, "Deployment", "spotter-"+name)
		grace, _ := get(d, "spec", "template", "spec", "terminationGracePeriodSeconds").(int)
		timeout, err := time.ParseDuration(str(envVar(containers(d)[0], "SHUTDOWN_TIMEOUT"), "value"))
		if err != nil || time.Duration(grace)*time.Second <= timeout {
			t.Errorf("%s: grace %ds, shutdown timeout %v (%v)", name, grace, timeout, err)
		}
	}
	threads := func(objs []object) string {
		return str(envVar(containers(find(objs, "Deployment", "spotter-worker"))[0], "MODEL_THREADS"), "value")
	}
	if got := threads(objs); got != "2" {
		t.Errorf("MODEL_THREADS = %q with a CPU limit of 2", got)
	}
	for limit, want := range map[string]string{"1500m": "1", "500m": "1", "3": "3"} {
		if got := threads(render(t, "worker.resources.limits.cpu="+limit)); got != want {
			t.Errorf("MODEL_THREADS = %q with a CPU limit of %s, want %s", got, limit, want)
		}
	}
}

func TestAutoscalingOn(t *testing.T) {
	spectest.Verifies(t, capability, "Workers scale on consumer lag")
	objs := render(t, "worker.autoscaling.enabled=true", "worker.autoscaling.minReplicas=1",
		"worker.autoscaling.maxReplicas=4", "worker.autoscaling.lagThreshold=20")
	so := find(objs, "ScaledObject", "spotter-worker")
	if so == nil {
		t.Fatal("no ScaledObject")
	}
	if str(so, "spec", "scaleTargetRef", "name") != "spotter-worker" ||
		get(so, "spec", "minReplicaCount") != 1 || get(so, "spec", "maxReplicaCount") != 4 {
		t.Fatalf("ScaledObject spec %v", get(so, "spec"))
	}
	trig := get(so, "spec", "triggers", "0")
	if str(trig, "type") != "nats-jetstream" || str(trig, "metadata", "stream") != "INFERENCE" ||
		str(trig, "metadata", "consumer") != "inference-worker-eurosat-classifier" || str(trig, "metadata", "lagThreshold") != "20" ||
		!strings.HasPrefix(str(trig, "metadata", "natsServerMonitoringEndpoint"), "spotter-nats.spotter.svc") {
		t.Fatalf("trigger %v", trig)
	}
	if w := get(so, "spec", "advanced", "horizontalPodAutoscalerConfig", "behavior", "scaleDown", "stabilizationWindowSeconds"); w != 120 {
		t.Errorf("scale-down stabilization %v, want the default 120", w)
	}
	if r := get(find(objs, "Deployment", "spotter-worker"), "spec", "replicas"); r != nil {
		t.Fatalf("worker Deployment sets replicas %v under KEDA", r)
	}
}

func TestAutoscalingOff(t *testing.T) {
	spectest.Verifies(t, capability, "Workers scale on consumer lag")
	objs := render(t, "worker.autoscaling.enabled=false", "worker.replicas=2")
	if len(ofKind(objs, "ScaledObject")) != 0 {
		t.Fatal("ScaledObject rendered with autoscaling off")
	}
	if r := get(find(objs, "Deployment", "spotter-worker"), "spec", "replicas"); r != 2 {
		t.Fatalf("worker replicas %v, want 2", r)
	}
}

func TestTrainerJob(t *testing.T) {
	spectest.Verifies(t, capability, "A trainer Job is available")
	if len(ofKind(render(t), "Job")) != 0 {
		t.Fatal("trainer Job rendered by default")
	}
	jobs := ofKind(render(t, "trainer.enabled=true"), "Job")
	if len(jobs) != 1 {
		t.Fatalf("%d Jobs, want 1", len(jobs))
	}
	c := containers(jobs[0])[0]
	if cmd := list(c, "command"); len(cmd) != 1 || cmd[0] != "spotter-train" {
		t.Errorf("command %v", cmd)
	}
	if p := str(jobs[0], "spec", "template", "spec", "restartPolicy"); p != "Never" {
		t.Errorf("restartPolicy %q", p)
	}
	for _, v := range []string{"S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		if str(envVar(c, v), "valueFrom", "secretKeyRef", "name") != "spotter-credentials" {
			t.Errorf("%s not from the Secret", v)
		}
	}
}

// ---- against a real cluster (CI's kind job, or k3d locally) ----

type cluster struct{ context, release, namespace string }

func kube(t *testing.T) cluster {
	t.Helper()
	ctx := os.Getenv("SPOTTER_KUBE_CONTEXT")
	if ctx == "" {
		t.Skip("set SPOTTER_KUBE_CONTEXT to the kube context of a cluster with the chart installed")
	}
	release, namespace := os.Getenv("SPOTTER_RELEASE"), os.Getenv("SPOTTER_NAMESPACE")
	if release == "" {
		release = "spotter"
	}
	if namespace == "" {
		namespace = "spotter"
	}
	return cluster{context: ctx, release: release, namespace: namespace}
}

func (c cluster) kubectl(t *testing.T, args ...string) string {
	t.Helper()
	return run(t, "kubectl", append([]string{"--context", c.context, "-n", c.namespace}, args...)...)
}

func (c cluster) helm(t *testing.T, args ...string) string {
	t.Helper()
	return run(t, "helm", append([]string{"--kube-context", c.context, "-n", c.namespace}, args...)...)
}

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestFreshInstall(t *testing.T) {
	spectest.Verifies(t, capability, "The platform installs into an empty cluster")
	c := kube(t)
	selector := "app.kubernetes.io/instance=" + c.release
	// Deployments may run several replicas (KEDA); the StatefulSets are single-instance.
	c.kubectl(t, "wait", "deployment", "-l", selector, "--for=condition=Available", "--timeout=5m")
	c.kubectl(t, "wait", "statefulset", "-l", selector, "--for=jsonpath={.status.readyReplicas}=1", "--timeout=5m")
	out := c.helm(t, "test", c.release, "--logs", "--timeout", "5m")
	for _, step := range []string{"ok: gateway ready", "ok: registry ready", "through the gateway", "back", "ok: worker answers a NATS ping"} {
		if !strings.Contains(out, step) {
			t.Errorf("smoke test output lacks %q:\n%s", step, out)
		}
	}
}

func TestUpgradeKeepsCredentials(t *testing.T) {
	spectest.Verifies(t, capability, "Credentials come from a generated Secret")
	c := kube(t)
	secret := func() string {
		return c.kubectl(t, "get", "secret", c.release+"-credentials", "-o", "jsonpath={.data}")
	}
	before := secret()
	c.helm(t, "upgrade", c.release, "spotter", "--reuse-values", "--wait", "--timeout", "5m")
	if after := secret(); after != before {
		t.Fatal("upgrade changed the generated credentials")
	}
}

func TestRetrainingJobAndWatcher(t *testing.T) {
	spectest.Verifies(t, "model-retraining", "The chart runs retraining on demand or on drift")
	if find(render(t), "Deployment", "spotter-retrainer") != nil {
		t.Fatal("retrainer rendered by default")
	}
	jobs := ofKind(render(t, "trainer.enabled=true", "trainer.command=spotter-retrain"), "Job")
	if len(jobs) != 1 {
		t.Fatalf("%d Jobs, want 1", len(jobs))
	}
	if cmd := list(containers(jobs[0])[0], "command"); len(cmd) != 1 || cmd[0] != "spotter-retrain" {
		t.Errorf("Job command %v, want [spotter-retrain]", cmd)
	}

	d := find(render(t, "retrainer.enabled=true"), "Deployment", "spotter-retrainer")
	if d == nil {
		t.Fatal("no retrainer Deployment")
	}
	if r, _ := get(d, "spec", "replicas").(int); r != 1 {
		t.Errorf("replicas %v, want 1", get(d, "spec", "replicas"))
	}
	c := containers(d)[0]
	if cmd := list(c, "command"); len(cmd) != 2 || cmd[0] != "spotter-retrain" || cmd[1] != "--watch" {
		t.Errorf("command %v", cmd)
	}
	env := map[string]string{}
	for _, e := range list(c, "env") {
		env[str(e, "name")] = str(e, "value")
	}
	if env["NATS_URL"] == "" || env["REGISTRY_ADDR"] == "" {
		t.Errorf("env %v lacks NATS_URL or REGISTRY_ADDR", env)
	}
}
