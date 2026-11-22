package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/gateway/internal/app"
	"github.com/Pohi11/spotter/services/gateway/internal/domain"
)

const capability = "inference-api"

var ctx = context.Background()

// fakes records every call so tests can assert that nothing was stored or published.
type fakes struct {
	mu        sync.Mutex
	images    map[string][]byte
	jobs      map[uuid.UUID]domain.Job
	published []domain.Job
	models    map[string]bool
	pubErr    error
}

func newFakes() *fakes {
	return &fakes{images: map[string][]byte{}, jobs: map[uuid.UUID]domain.Job{}, models: map[string]bool{"eurosat-classifier": true}}
}

func (f *fakes) Put(_ context.Context, key string, data []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images[key] = data
	return "s3://images/" + key, nil
}
func (f *fakes) Create(_ context.Context, j domain.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs[j.ID] = j
	return nil
}
func (f *fakes) Get(_ context.Context, id uuid.UUID) (domain.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	return j, nil
}
func (f *fakes) MarkFailed(_ context.Context, id uuid.UUID, reason string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	j := f.jobs[id]
	j.Status, j.Error, j.CompletedAt = domain.StatusFailed, reason, at
	f.jobs[id] = j
	return nil
}
func (f *fakes) PublishRequested(_ context.Context, j domain.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pubErr != nil {
		return f.pubErr
	}
	f.published = append(f.published, j)
	return nil
}
func (f *fakes) ModelExists(_ context.Context, name string) (bool, error) { return f.models[name], nil }

type ids struct{}

func (ids) NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

type clock struct{}

func (clock) Now() time.Time { return time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC) }

func service(f *fakes) *app.Service {
	return app.NewService(app.Deps{Images: f, Jobs: f, Pub: f, Models: f, IDs: ids{}, Clock: clock{}})
}

func TestSubmitStoresRecordsAndPublishes(t *testing.T) {
	spectest.Verifies(t, capability, "Images are submitted as asynchronous jobs")
	f := newFakes()
	job, err := service(f).Submit(ctx, "eurosat-classifier", []byte("jpeg bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.StatusPending || job.ImageURI != "s3://images/eurosat-classifier/"+job.ID.String() {
		t.Fatalf("job = %+v", job)
	}
	if len(f.images) != 1 || len(f.published) != 1 || f.published[0].ID != job.ID {
		t.Fatalf("images=%d published=%v", len(f.images), f.published)
	}
	got, err := service(f).Get(ctx, job.ID)
	if err != nil || got.Status != domain.StatusPending {
		t.Fatalf("get = %+v, %v", got, err)
	}
}

func TestInvalidSubmissionsStoreNothing(t *testing.T) {
	for _, tc := range []struct {
		name, model string
		image       []byte
		field       string
	}{
		{"empty image", "eurosat-classifier", nil, "image"},
		{"oversized image", "eurosat-classifier", make([]byte, 2<<20), "image"},
		{"malformed model", "Bad Name", []byte("x"), "model_name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spectest.Verifies(t, capability, "Submissions are validated")
			f := newFakes()
			_, err := service(f).Submit(ctx, tc.model, tc.image)
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tc.field {
				t.Fatalf("err = %v, want validation error on %s", err, tc.field)
			}
			if len(f.images)+len(f.jobs)+len(f.published) != 0 {
				t.Fatal("an invalid submission stored or published something")
			}
		})
	}
}

func TestUnknownModelIsNotFoundAndStoresNothing(t *testing.T) {
	spectest.Verifies(t, capability, "Submissions are validated")
	f := newFakes()
	_, err := service(f).Submit(ctx, "no-such-model", []byte("x"))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if len(f.images)+len(f.jobs)+len(f.published) != 0 {
		t.Fatal("a submission for an unknown model stored or published something")
	}
}

func TestPublishFailureMarksTheJobFailed(t *testing.T) {
	f := newFakes()
	f.pubErr = errors.New("nats: no responders")
	_, err := service(f).Submit(ctx, "eurosat-classifier", []byte("x"))
	if !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	for _, j := range f.jobs {
		if j.Status != domain.StatusFailed {
			t.Fatalf("job left %s after a publish failure", j.Status)
		}
	}
}

func TestUnknownJobIsNotFound(t *testing.T) {
	spectest.Verifies(t, capability, "Job status can be read")
	if _, err := service(newFakes()).Get(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
