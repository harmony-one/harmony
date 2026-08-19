package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/strictdb"
)

func dirStats(root string) (uint64, uint64, error) {
	var files, bytesUsed uint64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			files++
			bytesUsed += uint64(info.Size())
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("inventory: walk %s: %w", root, err)
	}
	return files, bytesUsed, nil
}

func inventoryCmd() *cobra.Command {
	var (
		dbPath   string
		readOnly bool
		output   string
	)
	cmd := &cobra.Command{
		Use:   "inventory-db",
		Short: "Minimal namespace accounting: counts + logical bytes per prefix bucket (plan WS2, revision 14)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireGlobals(cmd); err != nil {
				return err
			}
			if !readOnly {
				return usageErr("--read-only is mandatory for inventory-db")
			}
			if dbPath == "" || output == "" {
				return usageErr("--db and --output are mandatory")
			}
			if err := requireAbsPaths("db", dbPath, "output", output); err != nil {
				return err
			}
			return runInventory(dbPath, output)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "absolute path to the source database (opened strictly read-only)")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "acknowledge read-only source open (mandatory)")
	cmd.Flags().StringVar(&output, "output", "", "inventory.json output path")
	return cmd
}

func runInventory(dbPath, output string) error {
	db, ro, err := dbopen.OpenSourceDatabase(dbPath)
	if err != nil {
		return err
	}
	defer ro.Close()

	type bucketAcc struct {
		count uint64
		bytes uint64
	}
	buckets := map[string]*bucketAcc{}
	var malformed []string
	var totalKeys, totalBytes uint64

	// Single keyspace pass with the longest-prefix, key-shape-aware
	// classifier. All un-prefixed 32-byte keys land in the single physical
	// bare-hash32 bucket; on an archival source an unresolved bare key is
	// reported, never fatal (plan §2.2.9 severity split).
	err = strictdb.ForEach(db, nil, func(key, value []byte) error {
		bucket := keys.Classify(key)
		acc, ok := buckets[bucket]
		if !ok {
			acc = &bucketAcc{}
			buckets[bucket] = acc
		}
		acc.count++
		acc.bytes += uint64(len(key) + len(value))
		totalKeys++
		totalBytes += uint64(len(key) + len(value))
		if bucket == keys.BucketMalformed && len(malformed) < 100 {
			malformed = append(malformed, hex.EncodeToString(key))
		}
		return nil
	})
	if err != nil {
		return ioErr(err)
	}

	meta, err := report.NewMeta(report.InventorySchemaV1, "inventory-db", flagNetwork, flagShard, toolVersion(), nil)
	if err != nil {
		return ioErr(err)
	}
	var st syscall.Stat_t
	if err := syscall.Stat(dbPath, &st); err != nil {
		return ioErr(fmt.Errorf("inventory: stat %s: %w", dbPath, err))
	}
	files, dbBytes, err := dirStats(dbPath)
	if err != nil {
		return ioErr(err)
	}
	rep := &report.InventoryReport{
		Meta: meta,
		Source: report.SourceIdentity{
			AbsolutePath: dbPath, DeviceID: uint64(st.Dev),
			FileCount: files, TotalBytes: dbBytes,
		},
		MalformedKeys: malformed,
		TotalKeys:     totalKeys,
		TotalBytes:    totalBytes,
	}
	names := make([]string, 0, len(buckets))
	for name := range buckets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		rep.Buckets = append(rep.Buckets, report.InventoryBucket{
			Bucket: name, Count: buckets[name].count, LogicalBytes: buckets[name].bytes,
		})
	}
	sum, err := report.WriteJSON(output, rep)
	if err != nil {
		return ioErr(err)
	}
	fmt.Printf("inventory-db: report written to %s (sha256 %s; %d keys, %d logical bytes)\n", output, sum, totalKeys, totalBytes)
	return nil
}
