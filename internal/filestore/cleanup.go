package filestore

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wzfukui/agent-native-im/internal/store"
)

// RunFileCleanup starts the file cleanup loop. It runs once after startupDelay,
// then repeats every interval. It handles both expired DB records and orphaned
// disk files that have no matching record.
func RunFileCleanup(ctx context.Context, st store.Store, filesDir string, interval, maxAge, startupDelay time.Duration) {
	slog.Info("file-cleanup: goroutine started",
		"interval", interval.String(),
		"maxAge", maxAge.String(),
		"startupDelay", startupDelay.String(),
	)

	// Wait before the first run to let the server finish starting up.
	select {
	case <-ctx.Done():
		slog.Info("file-cleanup: goroutine stopped before first run")
		return
	case <-time.After(startupDelay):
	}

	// First run immediately after the startup delay.
	runCleanupCycle(ctx, st, filesDir, maxAge)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("file-cleanup: goroutine stopped")
			return
		case <-ticker.C:
			runCleanupCycle(ctx, st, filesDir, maxAge)
		}
	}
}

// runCleanupCycle performs one full cleanup pass: expired records then orphaned files.
func runCleanupCycle(ctx context.Context, st store.Store, filesDir string, maxAge time.Duration) {
	cleanExpiredRecords(ctx, st, filesDir, maxAge)
	cleanOrphanedFiles(ctx, st, filesDir)
}

// cleanExpiredRecords removes file records older than maxAge and their disk files.
// It processes in batches to avoid loading too many records at once.
func cleanExpiredRecords(ctx context.Context, st store.Store, filesDir string, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	const batchSize = 100

	totalDeleted := 0
	for {
		records, err := st.ListExpiredFileRecords(ctx, cutoff, batchSize)
		if err != nil {
			slog.Error("file-cleanup: failed to query expired records", "error", err)
			return
		}
		if len(records) == 0 {
			break
		}

		for _, rec := range records {
			diskPath := filepath.Join(filesDir, rec.StoredName)

			slog.Info("file-cleanup: removing expired file",
				"id", rec.ID,
				"stored_name", rec.StoredName,
				"created_at", rec.CreatedAt,
			)

			// Remove disk file first.
			if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
				slog.Error("file-cleanup: failed to remove disk file",
					"path", diskPath, "error", err,
				)
				// Still delete the DB record — the file may already be gone.
			}

			if err := st.DeleteFileRecord(ctx, rec.ID); err != nil {
				slog.Error("file-cleanup: failed to delete record",
					"id", rec.ID, "error", err,
				)
				continue
			}
			totalDeleted++
		}

		// If we got fewer than batchSize, we're done.
		if len(records) < batchSize {
			break
		}
	}

	if totalDeleted > 0 {
		slog.Info("file-cleanup: expired file cleanup complete", "deleted", totalDeleted)
	}
}

// cleanOrphanedFiles scans the files directory for files that exist on disk
// but have no corresponding record in the database. These can occur from
// incomplete uploads, manual copies, or bugs.
func cleanOrphanedFiles(ctx context.Context, st store.Store, filesDir string) {
	knownNames, err := st.ListAllStoredNames(ctx)
	if err != nil {
		slog.Error("file-cleanup: failed to list stored names from DB", "error", err)
		return
	}

	knownSet := make(map[string]struct{}, len(knownNames))
	for _, name := range knownNames {
		knownSet[name] = struct{}{}
	}

	entries, err := os.ReadDir(filesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return // Directory doesn't exist yet, nothing to clean.
		}
		slog.Error("file-cleanup: failed to read files directory", "error", err)
		return
	}

	orphanCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if _, known := knownSet[name]; known {
			continue
		}

		diskPath := filepath.Join(filesDir, name)

		slog.Info("file-cleanup: removing orphaned file (no DB record)",
			"file", name,
		)

		if err := os.Remove(diskPath); err != nil {
			slog.Error("file-cleanup: failed to remove orphaned file",
				"path", diskPath, "error", err,
			)
			continue
		}
		orphanCount++
	}

	if orphanCount > 0 {
		slog.Info("file-cleanup: orphaned file cleanup complete", "deleted", orphanCount)
	}
}
