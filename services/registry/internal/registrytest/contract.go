// Package registrytest is the contract suite every ports.Repository adapter must pass.
// It runs the spec scenarios through app.Service, so a scenario that passes here passes
// for real, whichever adapter is plugged in.
package registrytest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/protoevents"
	"github.com/Pohi11/spotter/services/registry/internal/app"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

const (
	registry  = "model-registry"
	lifecycle = "model-lifecycle"
	// SHA is a valid checksum (sha256 of "1").
	SHA = "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"
)

// Clock ticks one millisecond per call, so times are distinct, ordered, and survive a
// round trip through Postgres (microsecond precision). Safe for concurrent use.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

func NewClock() *Clock { return &Clock{now: time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)} }

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(time.Millisecond)
	return c.now
}

// IDs generates UUIDv7s (time-ordered).
type IDs struct{}

func (IDs) NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

// Run runs every scenario against a fresh repository from newRepo.
func Run(t *testing.T, newRepo func(t *testing.T) ports.Repository) {
	h := func(t *testing.T) *harness {
		t.Helper()
		repo := newRepo(t)
		return &harness{t: t, repo: repo, svc: app.NewService(repo, protoevents.Encoder{}, IDs{}, NewClock())}
	}
	scenarios := []struct {
		name string
		run  func(t *testing.T, h *harness)
	}{
		{"ValidModelName", validModelName},
		{"MalformedModelName", malformedModelName},
		{"DuplicateModelName", duplicateModelName},
		{"ValidVersion", validVersion},
		{"InvalidVersionStoresNothing", invalidVersionStoresNothing},
		{"SequentialNumbers", sequentialNumbers},
		{"ConcurrentNumbers", concurrentNumbers},
		{"NumbersPerModel", numbersPerModel},
		{"UnknownModel", unknownModel},
		{"UnknownVersion", unknownVersion},
		{"NewestFirst", newestFirst},
		{"EveryListedTransition", everyListedTransition},
		{"IllegalTransitionChangesNothing", illegalTransitionChangesNothing},
		{"TerminalStages", terminalStages},
		{"BlankReason", blankReason},
		{"HistoryInOrder", historyInOrder},
		{"OneProductionPerModel", oneProductionPerModel},
		{"ProductionPerModelIsIndependent", productionPerModelIsIndependent},
		{"StaleRevision", staleRevision},
		{"StaleRevisionAtRepository", staleRevisionAtRepository},
		{"ConcurrentTransitionsOneWinner", concurrentTransitionsOneWinner},
		{"RegistrationEvent", registrationEvent},
		{"TransitionEvent", transitionEvent},
		{"FailedTransitionsStoreNoEvent", failedTransitionsStoreNoEvent},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			sc.run(t, h(t))
		})
	}
}

type harness struct {
	t    *testing.T
	repo ports.Repository
	svc  *app.Service
}

var ctx = context.Background()

func (h *harness) model(name string) domain.Model {
	h.t.Helper()
	m, err := h.svc.RegisterModel(ctx, name, "test model")
	if err != nil {
		h.t.Fatalf("register model %q: %v", name, err)
	}
	return m
}

func (h *harness) version(modelName string) domain.ModelVersion {
	h.t.Helper()
	v, err := h.svc.RegisterVersion(ctx, app.RegisterVersionInput{
		ModelName: modelName, ArtifactURI: "s3://models/" + modelName + "/model.pt",
		ArtifactSHA256: SHA, Framework: domain.FrameworkPyTorch, Metrics: map[string]float64{"accuracy": 0.91},
	})
	if err != nil {
		h.t.Fatalf("register version of %q: %v", modelName, err)
	}
	return v
}

// move walks a version through stages, each from its current revision.
func (h *harness) move(v domain.ModelVersion, stages ...domain.Stage) domain.ModelVersion {
	h.t.Helper()
	for _, s := range stages {
		next, err := h.svc.TransitionVersion(ctx, v.ID, s, "test move to "+string(s), v.Revision)
		if err != nil {
			h.t.Fatalf("move %s → %s: %v", v.Stage, s, err)
		}
		v = next
	}
	return v
}

func (h *harness) stored(id uuid.UUID) domain.ModelVersion {
	h.t.Helper()
	v, err := h.svc.Version(ctx, id)
	if err != nil {
		h.t.Fatal(err)
	}
	return v
}

func (h *harness) outbox() []ports.OutboxRecord {
	h.t.Helper()
	recs, err := h.repo.PendingOutbox(ctx, 1000)
	if err != nil {
		h.t.Fatal(err)
	}
	return recs
}

func (h *harness) history(id uuid.UUID) []domain.StageChange {
	h.t.Helper()
	hist, err := h.svc.History(ctx, id)
	if err != nil {
		h.t.Fatal(err)
	}
	return hist
}

func wantField(t *testing.T, err error, field string) {
	t.Helper()
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != field {
		t.Fatalf("err = %v, want validation error on %q", err, field)
	}
}

// ---- model-registry ----

func validModelName(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Model names are unique and well-formed")
	m := h.model("eurosat-classifier")
	got, err := h.repo.ModelByName(ctx, "eurosat-classifier")
	if err != nil || got.ID != m.ID {
		t.Fatalf("find by name: %+v, %v", got, err)
	}
}

func malformedModelName(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Model names are unique and well-formed")
	_, err := h.svc.RegisterModel(ctx, "EuroSAT Classifier", "")
	wantField(t, err, "name")
	if _, err := h.repo.ModelByName(ctx, "EuroSAT Classifier"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("malformed model was stored: %v", err)
	}
}

func duplicateModelName(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Model names are unique and well-formed")
	h.model("eurosat-classifier")
	if _, err := h.svc.RegisterModel(ctx, "eurosat-classifier", ""); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
}

func validVersion(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "A version records a verifiable artifact")
	h.model("eurosat-classifier")
	v := h.version("eurosat-classifier")
	got := h.stored(v.ID)
	if got.Stage != domain.StageRegistered || got.Revision != 1 || got.Metrics["accuracy"] != 0.91 ||
		got.ArtifactSHA256 != SHA || got.Framework != domain.FrameworkPyTorch {
		t.Fatalf("stored version = %+v", got)
	}
}

func invalidVersionStoresNothing(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "A version records a verifiable artifact")
	h.model("eurosat-classifier")
	for _, in := range []struct {
		field string
		input app.RegisterVersionInput
	}{
		{"artifact_sha256", app.RegisterVersionInput{ArtifactURI: "s3://m/k", ArtifactSHA256: "abc123", Framework: domain.FrameworkPyTorch}},
		{"framework", app.RegisterVersionInput{ArtifactURI: "s3://m/k", ArtifactSHA256: SHA, Framework: "tensorflow"}},
		{"artifact_uri", app.RegisterVersionInput{ArtifactURI: "/tmp/model.pt", ArtifactSHA256: SHA, Framework: domain.FrameworkPyTorch}},
	} {
		in.input.ModelName = "eurosat-classifier"
		_, err := h.svc.RegisterVersion(ctx, in.input)
		wantField(t, err, in.field)
	}
	if vs, _ := h.svc.Versions(ctx, "eurosat-classifier"); len(vs) != 0 {
		t.Fatalf("invalid versions were stored: %+v", vs)
	}
	if n := len(h.outbox()); n != 0 {
		t.Fatalf("invalid versions stored %d events", n)
	}
}

func sequentialNumbers(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Version numbers are sequential per model")
	h.model("eurosat-classifier")
	if a, b := h.version("eurosat-classifier"), h.version("eurosat-classifier"); a.Number != 1 || b.Number != 2 {
		t.Fatalf("numbers = %d, %d; want 1, 2", a.Number, b.Number)
	}
}

func concurrentNumbers(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Version numbers are sequential per model")
	h.model("eurosat-classifier")
	const n = 10
	numbers := make(chan int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range n {
		wg.Go(func() {
			<-start
			v, err := h.svc.RegisterVersion(ctx, app.RegisterVersionInput{
				ModelName: "eurosat-classifier", ArtifactURI: "s3://m/k", ArtifactSHA256: SHA, Framework: domain.FrameworkONNX,
			})
			if err != nil {
				t.Errorf("concurrent register: %v", err)
				return
			}
			numbers <- v.Number
		})
	}
	close(start)
	wg.Wait()
	close(numbers)
	seen := map[int]bool{}
	for num := range numbers {
		seen[num] = true
	}
	for i := 1; i <= n; i++ {
		if !seen[i] {
			t.Fatalf("numbers %v are not exactly 1..%d", seen, n)
		}
	}
}

func numbersPerModel(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Version numbers are sequential per model")
	h.model("model-a")
	h.model("model-b")
	if a, b := h.version("model-a"), h.version("model-b"); a.Number != 1 || b.Number != 1 {
		t.Fatalf("numbers = %d, %d; want 1, 1", a.Number, b.Number)
	}
}

func unknownModel(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Unknown models and versions are reported as not found")
	_, err := h.svc.RegisterVersion(ctx, app.RegisterVersionInput{
		ModelName: "no-such-model", ArtifactURI: "s3://m/k", ArtifactSHA256: SHA, Framework: domain.FrameworkPyTorch,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func unknownVersion(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Unknown models and versions are reported as not found")
	if _, err := h.svc.Version(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := h.svc.TransitionVersion(ctx, uuid.New(), domain.StageValidated, "x", 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("transition of unknown version: err = %v, want ErrNotFound", err)
	}
}

func newestFirst(t *testing.T, h *harness) {
	spectest.Verifies(t, registry, "Versions are listed newest first")
	h.model("eurosat-classifier")
	for range 3 {
		h.version("eurosat-classifier")
	}
	vs, err := h.svc.Versions(ctx, "eurosat-classifier")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 3 || vs[0].Number != 3 || vs[1].Number != 2 || vs[2].Number != 1 {
		t.Fatalf("got numbers %v", numbersOf(vs))
	}
}

func numbersOf(vs []domain.ModelVersion) []int {
	out := make([]int, len(vs))
	for i, v := range vs {
		out[i] = v.Number
	}
	return out
}

// ---- model-lifecycle ----

func everyListedTransition(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Only listed stage transitions are allowed")
	h.model("eurosat-classifier")
	// Covers all nine listed transitions across three versions.
	a := h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageShadow, domain.StageProduction,
		domain.StageRetired, domain.StageProduction, domain.StageRolledBack)
	h.move(h.version("eurosat-classifier"), domain.StageRejected)
	h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageRejected)
	c := h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageShadow, domain.StageRejected)
	if a.Stage != domain.StageRolledBack || c.Stage != domain.StageRejected {
		t.Fatalf("final stages %s, %s", a.Stage, c.Stage)
	}
}

func illegalTransitionChangesNothing(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Only listed stage transitions are allowed")
	spectest.Verifies(t, lifecycle, "Every transition is recorded in the version's history")
	h.model("eurosat-classifier")
	v := h.version("eurosat-classifier")
	for _, to := range []domain.Stage{domain.StageProduction, domain.StageRetired, domain.StageRegistered} {
		if _, err := h.svc.TransitionVersion(ctx, v.ID, to, "skip ahead", v.Revision); !errors.Is(err, domain.ErrIllegalTransition) {
			t.Fatalf("registered → %s: err = %v, want illegal transition", to, err)
		}
	}
	if got := h.stored(v.ID); got.Stage != domain.StageRegistered || got.Revision != 1 {
		t.Fatalf("version changed: %+v", got)
	}
	if hist := h.history(v.ID); len(hist) != 0 {
		t.Fatalf("history = %+v, want empty", hist)
	}
}

func terminalStages(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Rejected and rolled-back versions are final")
	h.model("eurosat-classifier")
	rejected := h.move(h.version("eurosat-classifier"), domain.StageRejected)
	rolledBack := h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageShadow, domain.StageProduction, domain.StageRolledBack)
	for _, v := range []domain.ModelVersion{rejected, rolledBack} {
		for _, to := range domain.Stages() {
			if _, err := h.svc.TransitionVersion(ctx, v.ID, to, "revive", v.Revision); !errors.Is(err, domain.ErrIllegalTransition) {
				t.Fatalf("%s → %s: err = %v, want illegal transition", v.Stage, to, err)
			}
		}
	}
}

func blankReason(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "A transition requires a reason")
	h.model("eurosat-classifier")
	v := h.version("eurosat-classifier")
	_, err := h.svc.TransitionVersion(ctx, v.ID, domain.StageValidated, "   ", v.Revision)
	wantField(t, err, "reason")
}

func historyInOrder(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Every transition is recorded in the version's history")
	h.model("eurosat-classifier")
	v := h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageShadow)
	hist := h.history(v.ID)
	if len(hist) != 2 {
		t.Fatalf("history = %+v, want 2 entries", hist)
	}
	if hist[0].From != domain.StageRegistered || hist[0].To != domain.StageValidated || hist[0].Revision != 2 ||
		hist[1].From != domain.StageValidated || hist[1].To != domain.StageShadow || hist[1].Revision != 3 {
		t.Fatalf("history = %+v", hist)
	}
	if hist[0].Reason == "" || !hist[0].At.Before(hist[1].At) {
		t.Fatalf("history entries missing reason or out of time order: %+v", hist)
	}
}

func oneProductionPerModel(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "At most one production version per model")
	h.model("eurosat-classifier")
	h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageShadow, domain.StageProduction)
	v2 := h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageShadow)

	_, err := h.svc.TransitionVersion(ctx, v2.ID, domain.StageProduction, "promote", v2.Revision)
	if !errors.Is(err, domain.ErrProductionOccupied) {
		t.Fatalf("err = %v, want ErrProductionOccupied", err)
	}
	if got := h.stored(v2.ID); got.Stage != domain.StageShadow || got.Revision != v2.Revision {
		t.Fatalf("version 2 changed: %+v", got)
	}
}

func productionPerModelIsIndependent(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "At most one production version per model")
	h.model("model-a")
	h.model("model-b")
	h.move(h.version("model-a"), domain.StageValidated, domain.StageShadow, domain.StageProduction)
	h.move(h.version("model-b"), domain.StageValidated, domain.StageShadow, domain.StageProduction)
}

func staleRevision(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Stale transitions are rejected")
	h.model("eurosat-classifier")
	v := h.move(h.version("eurosat-classifier"), domain.StageValidated) // now revision 2
	if _, err := h.svc.TransitionVersion(ctx, v.ID, domain.StageShadow, "late", 1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

// The repository must re-check the revision itself, not rely on the use case's early check.
func staleRevisionAtRepository(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Stale transitions are rejected")
	h.model("eurosat-classifier")
	v := h.version("eurosat-classifier")
	next, change, err := v.Transition(domain.StageValidated, "a", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.SaveTransition(ctx, next, 1, change, nil); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := h.repo.SaveTransition(ctx, next, 1, change, nil); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second save with the same expected revision: err = %v, want ErrConflict", err)
	}
}

func concurrentTransitionsOneWinner(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Stale transitions are rejected")
	h.model("eurosat-classifier")
	v := h.version("eurosat-classifier")

	results := make(chan error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, to := range []domain.Stage{domain.StageValidated, domain.StageRejected} {
		wg.Go(func() {
			<-start
			_, err := h.svc.TransitionVersion(ctx, v.ID, to, "race", 1)
			results <- err
		})
	}
	close(start)
	wg.Wait()
	close(results)

	wins, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, domain.ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins = %d, conflicts = %d; want 1 and 1", wins, conflicts)
	}
	if hist := h.history(v.ID); len(hist) != 1 {
		t.Fatalf("history has %d entries, want 1", len(hist))
	}
}

func registrationEvent(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Changes and their events are stored together")
	h.model("eurosat-classifier")
	v := h.version("eurosat-classifier")
	recs := h.outbox()
	if len(recs) != 1 || recs[0].Type != "spotter.v1.ModelVersionRegistered" || recs[0].AggregateID != v.ID {
		t.Fatalf("outbox = %+v", recs)
	}
	var ev spotterv1.ModelVersionRegistered
	if err := proto.Unmarshal(recs[0].Payload, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.GetVersionId() != v.ID.String() || ev.GetModelName() != "eurosat-classifier" || ev.GetVersion() != 1 ||
		ev.GetArtifactUri() != v.ArtifactURI || ev.GetArtifactSha256() != SHA || ev.GetEventId() != recs[0].ID.String() {
		t.Fatalf("decoded event = %v", &ev)
	}
}

func transitionEvent(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Changes and their events are stored together")
	h.model("eurosat-classifier")
	v := h.move(h.version("eurosat-classifier"), domain.StageValidated)
	recs := h.outbox()
	if len(recs) != 2 || recs[1].Type != "spotter.v1.ModelVersionStageChanged" {
		t.Fatalf("outbox = %+v", recs)
	}
	if recs[0].ID == recs[1].ID {
		t.Fatal("two events share an id")
	}
	var ev spotterv1.ModelVersionStageChanged
	if err := proto.Unmarshal(recs[1].Payload, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.GetFromStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_REGISTERED ||
		ev.GetToStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED ||
		ev.GetRevision() != 2 || ev.GetReason() == "" || ev.GetVersionId() != v.ID.String() {
		t.Fatalf("decoded event = %v", &ev)
	}
}

func failedTransitionsStoreNoEvent(t *testing.T, h *harness) {
	spectest.Verifies(t, lifecycle, "Changes and their events are stored together")
	h.model("eurosat-classifier")
	h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageShadow, domain.StageProduction)
	v2 := h.move(h.version("eurosat-classifier"), domain.StageValidated, domain.StageShadow)
	before := len(h.outbox())

	attempts := []struct {
		name string
		to   domain.Stage
		rev  int64
	}{
		{"illegal", domain.StageRegistered, v2.Revision},
		{"stale", domain.StageRejected, v2.Revision - 1},
		{"production occupied", domain.StageProduction, v2.Revision},
	}
	for _, a := range attempts {
		if _, err := h.svc.TransitionVersion(ctx, v2.ID, a.to, "should fail", a.rev); err == nil {
			t.Fatalf("%s transition succeeded", a.name)
		}
	}
	if after := len(h.outbox()); after != before {
		t.Fatalf("outbox grew from %d to %d after failed transitions", before, after)
	}
}
