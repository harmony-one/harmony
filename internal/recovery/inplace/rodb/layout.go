package rodb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckLayout gates on the simple default layout: a single-directory
// goleveldb database whose basename is exactly harmony_db_<shard>. Sharded
// multi-LevelDB (harmony_sharddb_*), pebble and TiKV layouts have no safe
// read-only open path here, and a wrong-shard or renamed directory would
// produce confusing FAILs instead of a clear refusal; all are rejected with
// a *LayoutError (exit code 2).
func CheckLayout(dir string, shardID uint32) error {
	st, err := os.Stat(dir)
	if err != nil {
		return &LayoutError{Reason: fmt.Sprintf("cannot stat --db path: %v", err)}
	}
	if !st.IsDir() {
		return &LayoutError{Reason: "--db path is not a directory"}
	}
	base := filepath.Base(filepath.Clean(dir))
	if strings.HasPrefix(base, "harmony_sharddb") {
		return &LayoutError{Reason: "sharded multi-LevelDB layout (harmony_sharddb*) is not supported; only the default harmony_db_0 LevelDB is supported"}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return &LayoutError{Reason: fmt.Sprintf("cannot list --db directory: %v", err)}
	}
	var hasCurrent, hasManifest, hasShardSubdir, hasDBSubdir bool
	for _, ent := range entries {
		name := ent.Name()
		switch {
		case strings.HasPrefix(name, "OPTIONS"):
			return &LayoutError{Reason: "found OPTIONS file: this looks like a pebble database, which is not supported; only the default harmony_db_0 LevelDB is supported"}
		case name == "CURRENT":
			hasCurrent = true
		case strings.HasPrefix(name, "MANIFEST-"):
			hasManifest = true
		case ent.IsDir() && strings.HasPrefix(name, "harmony_sharddb"):
			hasShardSubdir = true
		case ent.IsDir() && strings.HasPrefix(name, "harmony_db_"):
			hasDBSubdir = true
		}
	}
	if hasShardSubdir {
		return &LayoutError{Reason: "directory contains a harmony_sharddb* database; the sharded layout is not supported"}
	}
	want := fmt.Sprintf("harmony_db_%d", shardID)
	if base != want {
		reason := fmt.Sprintf("--db must point at the node's %s directory itself (basename is %q)", want, base)
		if hasDBSubdir {
			reason += fmt.Sprintf("; did you mean the %s subdirectory?", want)
		}
		return &LayoutError{Reason: reason}
	}
	if !hasCurrent || !hasManifest {
		reason := "not a LevelDB database directory (missing CURRENT/MANIFEST)"
		if hasDBSubdir {
			reason += "; did you mean to pass the harmony_db_0 subdirectory?"
		}
		return &LayoutError{Reason: reason}
	}
	return nil
}
