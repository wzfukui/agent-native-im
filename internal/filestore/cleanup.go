package filestore

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wzfukui/agent-native-im/internal/store"
)

// CleanExpiredFiles periodically removes file records and their disk files
// that are older than maxAge. It runs in a loop with the given interval.
func CleanExpiredFiles(ctx context.Context, st store.Store, filesDir string, interval, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("filestore: cleanup goroutine started (interval=%v, maxAge=%v)", interval, maxAge)

	for {
		select {
		case <-ctx.Done():
			log.Println("filestore: cleanup goroutine stopped")
			return
		case <-ticker.C:
			cleanOnce(ctx, st, filesDir, maxAge)
		}
	}
}

func cleanOnce(ctx context.Context, st store.Store, filesDir string, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	const batchSize = 100

	records, err := st.ListExpiredFileRecords(ctx, cutoff, batchSize)
	if err != nil {
		log.Printf("filestore: cleanup query failed: %v", err)
		return
	}

	if len(records) == 0 {
		return
	}

	deleted := 0
	for _, rec := range records {
		// Remove disk file
		diskPath := filepath.Join(filesDir, rec.StoredName)
		if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
			log.Printf("filestore: failed to remove file %s: %v", diskPath, err)
			// Continue to delete DB record even if disk file is already gone
		}

		// Remove DB record
		if err := st.DeleteFileRecord(ctx, rec.ID); err != nil {
			log.Printf("filestore: failed to delete record %d: %v", rec.ID, err)
			continue
		}
		deleted++
	}

	if deleted > 0 {
		log.Printf("filestore: cleaned up %d expired files", deleted)
	}
}
