// Package report provides the structured JSON report writers, the shared
// DigestSet schema, the journal state machine and the durability helpers for
// harmony-recovery-db (plan WS1).
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
)

// Meta is the common header every phase report carries: producing command
// and tool identity, plus the hash-chain links of every input the command
// consumed (plan §4 "Integrity and hash-chaining").
type Meta struct {
	SchemaVersion string               `json:"schema_version"`
	Command       string               `json:"command"`
	Network       string               `json:"network"`
	ShardID       uint32               `json:"shard_id"`
	ToolVersion   string               `json:"tool_version"`
	ToolBinary    string               `json:"tool_binary_sha256"`
	CreatedAt     string               `json:"created_at"`
	Inputs        []integrity.InputRef `json:"inputs"`
}

// NewMeta fills a Meta for a command, hashing the running binary.
func NewMeta(schema, command, network string, shardID uint32, toolVersion string, inputs []integrity.InputRef) (Meta, error) {
	self, err := integrity.SelfSHA256()
	if err != nil {
		return Meta{}, err
	}
	return Meta{
		SchemaVersion: schema,
		Command:       command,
		Network:       network,
		ShardID:       shardID,
		ToolVersion:   toolVersion,
		ToolBinary:    self,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Inputs:        inputs,
	}, nil
}

// WriteJSON marshals v (indented, deterministic field order via struct
// definitions), writes it atomically-ish (temp + rename in the same
// directory), fsyncs file and directory, writes the sibling .sha256, and
// returns the file's SHA-256.
func WriteJSON(path string, v interface{}) (string, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("report: marshal %s: %w", path, err)
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-report-*")
	if err != nil {
		return "", fmt.Errorf("report: temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { os.Remove(tmpName) }
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		cleanup()
		return "", fmt.Errorf("report: write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return "", fmt.Errorf("report: fsync %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf("report: close %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return "", fmt.Errorf("report: rename into %s: %w", path, err)
	}
	if err := fsyncDir(dir); err != nil {
		return "", err
	}
	sum, err := integrity.WriteChecksumFile(path)
	if err != nil {
		return "", err
	}
	return sum, nil
}

// ReadJSONStrict decodes a JSON file into v with unknown fields rejected.
func ReadJSONStrict(path string, v interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("report: read %s: %w", path, err)
	}
	dec := json.NewDecoder(newBytesReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("report: strict decode %s: %w", path, err)
	}
	if dec.More() {
		return fmt.Errorf("report: trailing data after JSON document in %s", path)
	}
	return nil
}
