package repository

import (
        "context"
        "testing"
	"io"

        "github.com/restic/restic/internal/backend"
        "github.com/restic/restic/internal/restic"
)

// This documents the observable state of the intentionally induced double
// Save failure; it does not declare whether backup-only recovery is acceptable.
func TestPhase3FailedUpgradeRollbackLeavesNoConfig(t *testing.T) {
        be := TestBackend(t)
        be = &failBackend{ConfigFileSavesUntilError: 1, Backend: be}
        repo, _ := TestRepositoryWithBackend(t, be, 1, Options{})
        if err := UpgradeRepo(context.Background(), repo); err == nil {
                t.Fatal("UpgradeRepo unexpectedly succeeded")
        }
        if _, err := repo.LoadRaw(context.Background(), restic.ConfigFile, restic.ID{}); err == nil {
                t.Fatal("config remains readable after induced failed rollback")
        }
        // The direct backend check ensures this is not only Repository cache state.
        if err := be.Load(context.Background(), backend.Handle{Type: backend.ConfigFile}, 0, 0, func(io.Reader) error { return nil }); err == nil {
                t.Fatal("backend config remains after induced failed rollback")
        }
}
