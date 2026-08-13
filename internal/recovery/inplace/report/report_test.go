package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReportPath(t *testing.T) {
	db := t.TempDir()
	if err := ValidateReportPath(filepath.Join(db, "r.json"), db); err == nil {
		t.Fatal("path inside the DB directory must be refused")
	}
	if err := ValidateReportPath(filepath.Join(db, "sub", "r.json"), db); err == nil {
		t.Fatal("nested path inside the DB directory must be refused")
	}
	outside := filepath.Join(t.TempDir(), "r.json")
	if err := ValidateReportPath(outside, db); err != nil {
		t.Fatalf("outside path refused: %v", err)
	}
	// A symlinked parent that resolves into the DB directory is refused.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(db, link); err == nil {
		if err := ValidateReportPath(filepath.Join(link, "r.json"), db); err == nil {
			t.Fatal("symlinked path into the DB directory must be refused")
		}
	}
}

func TestReceiptAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "receipt.json")
	rec := &Receipt{
		Tool:   Tool,
		Schema: Schema,
		Checks: NewChecks(),
		Result: "PASS",
	}
	if err := rec.Write(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back Receipt
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("written receipt does not parse: %v", err)
	}
	if back.Schema != Schema {
		t.Fatalf("schema %q", back.Schema)
	}
	// No temp files left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("stray files after atomic write: %v", entries)
	}
	for _, id := range CheckIDs {
		if back.Checks[id] != "skipped" {
			t.Fatalf("check %s = %q", id, back.Checks[id])
		}
	}
}

func TestFinalLine(t *testing.T) {
	var buf bytes.Buffer
	FinalLine(&buf, true, "")
	if buf.String() != "PASS\n" {
		t.Fatalf("pass line %q", buf.String())
	}
	buf.Reset()
	FinalLine(&buf, false, "target_header: gone")
	if buf.String() != "FAIL: target_header: gone\n" {
		t.Fatalf("fail line %q", buf.String())
	}
	if strings.Count(buf.String(), "\n") != 1 {
		t.Fatal("final line must be exactly one line")
	}
}

func TestFailureError(t *testing.T) {
	f := Failf("body", "root mismatch %d", 42)
	if f.Error() != "body: root mismatch 42" || !f.VerificationFailure() {
		t.Fatalf("%v", f)
	}
}
