// Package s3test gives tests a real S3-compatible store (SeaweedFS).
//
// The first call in a test binary starts one SeaweedFS container with the same image
// and entrypoint as deploy/compose; set S3TEST_ENDPOINT (plus S3_ACCESS_KEY and
// S3_SECRET_KEY) to use a running store instead.
package s3test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Image matches deploy/compose/compose.yaml.
const Image = "chrislusf/seaweedfs:4.54"

// Store is where tests reach the S3 API.
type Store struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
}

var (
	once    sync.Once
	store   Store
	initErr error
)

func start() (Store, error) {
	once.Do(func() {
		if ep := os.Getenv("S3TEST_ENDPOINT"); ep != "" {
			store = Store{Endpoint: ep, AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), Region: "us-east-1"}
			return
		}
		entrypoint, err := repoFile("deploy/compose/seaweedfs-entrypoint.sh")
		if err != nil {
			initErr = err
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		var c *testcontainers.DockerContainer
		// Retry: testcontainers' shared reaper can be mid-shutdown when several test
		// binaries start containers at once (seen locally with pgtest).
		for attempt := range 3 {
			c, err = testcontainers.Run(ctx, Image,
				testcontainers.WithEnv(map[string]string{"S3_ACCESS_KEY": "test-key", "S3_SECRET_KEY": "test-secret"}),
				testcontainers.WithFiles(testcontainers.ContainerFile{HostFilePath: entrypoint, ContainerFilePath: "/entrypoint-spotter.sh", FileMode: 0o755}),
				testcontainers.WithEntrypoint("/bin/sh", "/entrypoint-spotter.sh"),
				testcontainers.WithExposedPorts("8333/tcp"),
				testcontainers.WithWaitStrategy(wait.ForHTTP("/healthz").WithPort("8333/tcp").WithStartupTimeout(2*time.Minute)),
			)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
		if err != nil {
			initErr = fmt.Errorf("start seaweedfs: %w", err)
			return
		}
		endpoint, err := c.PortEndpoint(ctx, "8333/tcp", "http")
		if err != nil {
			initErr = err
			return
		}
		store = Store{Endpoint: endpoint, AccessKey: "test-key", SecretKey: "test-secret", Region: "us-east-1"}
	})
	return store, initErr
}

// Start returns the shared store, starting it on first use.
func Start(t testing.TB) Store {
	t.Helper()
	s, err := start()
	if err != nil {
		t.Fatalf("s3test: %v", err)
	}
	return s
}

// BucketName returns a bucket name unique to this test.
func BucketName() string { return "test-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16] }

// repoFile finds a file relative to the repository root (the directory holding go.mod).
func repoFile(rel string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, filepath.FromSlash(rel)), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above the working directory")
		}
		dir = parent
	}
}
