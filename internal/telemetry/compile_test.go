package telemetry

import (
	"github.com/steveyegge/beads/internal/storage"
)

// Compile-time interface compliance assertion (BUG-23).
// This line produces a build error if InstrumentedStorage's method set
// drifts from the storage.Storage interface (e.g., RunInTransaction
// gains a commitMsg parameter but the wrapper is not updated).
var _ storage.Storage = (*InstrumentedStorage)(nil)
