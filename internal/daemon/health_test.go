package daemon

import (
	"path/filepath"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

func TestHealthStatsIncludesOpenClawAdapter(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "shadow.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	stats, err := GetHealthStats(db)
	if err != nil {
		t.Fatalf("GetHealthStats: %v", err)
	}

	for _, sync := range stats.AdapterSyncs {
		if sync.AgentName == "openclaw" {
			return
		}
	}
	t.Fatalf("adapter syncs = %#v, want openclaw", stats.AdapterSyncs)
}
