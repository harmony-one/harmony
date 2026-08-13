package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/bundle"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

func exportCmd() *cobra.Command {
	var (
		sourceDB        string
		readOnly        bool
		baselinePath    string
		fromHeight      uint64
		toHeight        uint64
		certChildHeight uint64
		reportOnly      bool
		output          string
		anchorPath      string
		chunkBytes      int64
		donor           string
	)
	cmd := &cobra.Command{
		Use:   "export-bundle",
		Short: "Single-donor block+certificate export with mechanical donor preflight (plan WS3)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireGlobals(cmd); err != nil {
				return err
			}
			if !readOnly {
				return usageErr("--read-only is mandatory for export-bundle (donors are readable reservoirs, never trusted as DBs)")
			}
			if sourceDB == "" || output == "" || baselinePath == "" {
				return usageErr("--source-db, --baseline-manifest and --output are mandatory")
			}
			if fromHeight == 0 || toHeight == 0 || certChildHeight == 0 {
				return usageErr("--from-height, --to-height and --certificate-child-height are mandatory")
			}
			if err := requireAbsPaths("source-db", sourceDB, "baseline-manifest", baselinePath,
				"output", output, "anchor-manifest", anchorPath); err != nil {
				return err
			}
			return runExport(exportParams{
				sourceDB: sourceDB, baselinePath: baselinePath,
				fromHeight: fromHeight, toHeight: toHeight, certChildHeight: certChildHeight,
				reportOnly: reportOnly, output: output, anchorPath: anchorPath,
				chunkBytes: chunkBytes, donor: donor,
			})
		},
	}
	cmd.Flags().StringVar(&sourceDB, "source-db", "", "absolute path to the donor harmony_db_0 (stopped copy or crash-consistent snapshot; opened strictly read-only)")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "acknowledge read-only source open (mandatory)")
	cmd.Flags().StringVar(&baselinePath, "baseline-manifest", "", "the baseline copy's inspect report (from-height must be its head+1)")
	cmd.Flags().Uint64Var(&fromHeight, "from-height", 0, "first exported block (baseline head + 1)")
	cmd.Flags().Uint64Var(&toHeight, "to-height", 0, "last exported block (the pinned target)")
	cmd.Flags().Uint64Var(&certChildHeight, "certificate-child-height", 0, "abandoned child height carrying the target certificate (target+1)")
	cmd.Flags().BoolVar(&reportOnly, "report-only", false, "run only the mechanical donor preflight")
	cmd.Flags().StringVar(&output, "output", "", "bundle output directory (or preflight report path with --report-only)")
	cmd.Flags().StringVar(&anchorPath, "anchor-manifest", "", "optional anchor manifest (pinned-hash assertions)")
	cmd.Flags().Int64Var(&chunkBytes, "chunk-bytes", bundle.DefaultChunkBytes, "chunk size")
	cmd.Flags().StringVar(&donor, "donor", "", "donor identity string recorded in the manifest")
	return cmd
}

type exportParams struct {
	sourceDB        string
	baselinePath    string
	fromHeight      uint64
	toHeight        uint64
	certChildHeight uint64
	reportOnly      bool
	output          string
	anchorPath      string
	chunkBytes      int64
	donor           string
}

func runExport(p exportParams) error {
	// Baseline inspect report (checksum-gated): from-height must equal its
	// head+1 (plan WS3 acceptance: wrong --from-height refuses).
	if _, err := integrity.VerifyChecksumFile(p.baselinePath); err != nil {
		return preconditionErr(err)
	}
	baselineRef, err := integrity.NewInputRef("baseline-manifest", p.baselinePath)
	if err != nil {
		return ioErr(err)
	}
	var baseline report.InspectReport
	if err := report.ReadJSONStrict(p.baselinePath, &baseline); err != nil {
		return preconditionErr(err)
	}
	if len(baseline.Heads) == 0 || !baseline.HeadsAgree {
		return preconditionErr(fmt.Errorf("export: baseline inspect report has no agreed head tuple"))
	}
	baseHead := baseline.Heads[0]

	inputs := []integrity.InputRef{baselineRef}
	var anc *anchor.Manifest
	if p.anchorPath != "" {
		if _, err := integrity.VerifyChecksumFile(p.anchorPath); err != nil {
			return preconditionErr(err)
		}
		ref, err := integrity.NewInputRef("anchor-manifest", p.anchorPath)
		if err != nil {
			return ioErr(err)
		}
		inputs = append(inputs, ref)
		if anc, err = anchor.Load(p.anchorPath); err != nil {
			return preconditionErr(err)
		}
	}

	// The certificate verification needs the schedule globals + chain
	// config for the donor's network.
	if _, err := harness.InitSchedule(flagNetwork); err != nil {
		return usageErr("%v", err)
	}
	chainConfig, err := harness.ChainConfig(flagNetwork, flagShard)
	if err != nil {
		return usageErr("%v", err)
	}

	db, ro, err := dbopen.OpenSourceDatabase(p.sourceDB)
	if err != nil {
		return err
	}
	defer ro.Close()

	cfg := bundle.ExportConfig{
		Network: flagNetwork, ShardID: flagShard, ChainConfig: chainConfig,
		FromHeight: p.fromHeight, ToHeight: p.toHeight, CertChildHeight: p.certChildHeight,
		BaselineHeight: baseHead.Height, BaselineHash: common.HexToHash(baseHead.Hash),
		Anchor: anc, OutputDir: p.output, ChunkBytes: p.chunkBytes,
		Donor: p.donor, ToolVersion: toolVersion(), Inputs: inputs,
	}

	if p.reportOnly {
		rep, err := bundle.Preflight(db, cfg)
		if err != nil {
			return preconditionErr(err)
		}
		if _, err := report.WriteJSON(p.output, rep); err != nil {
			return ioErr(err)
		}
		fmt.Printf("export-bundle: preflight report written to %s (passed=%v, gaps=%d)\n", p.output, rep.Passed, rep.GapCount)
		if !rep.Passed {
			return verificationErr(fmt.Errorf("donor preflight failed with %d gaps; a gapped donor is refused", rep.GapCount))
		}
		return nil
	}

	manifest, err := bundle.Export(db, cfg)
	if err != nil {
		return verificationErr(err)
	}
	fmt.Printf("export-bundle: %d records in %d chunks written to %s (ordered digest %s)\n",
		manifest.RecordCount, len(manifest.Chunks), p.output, manifest.OrderedHashDigest)
	return nil
}
