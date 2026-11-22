package memory_test

import (
	"testing"

	"github.com/Pohi11/spotter/services/registry/internal/adapters/memory"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
	"github.com/Pohi11/spotter/services/registry/internal/registrytest"
)

func TestContract(t *testing.T) {
	registrytest.Run(t, func(*testing.T) ports.Repository { return memory.New() })
}
