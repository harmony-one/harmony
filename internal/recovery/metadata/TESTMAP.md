# Metadata workstream — test map

Traces each in-scope B5 acceptance bullet (as amended by the §8 operator
decision log) to a named test. Deferred bullets are marked `B4`. Run the
whole suite with the BLS build environment sourced
(`scripts/setup_bls_build_flags.sh`) under `-race`.

## B5 mapping (in-scope subset)

| B5 bullet | Amendment | Test |
|-----------|-----------|------|
| (1) dirty-descendant normalizes to the clean golden | — | `acceptance.TestMechanicalCleanEquality`, `acceptance.TestJunkInsensitivity` |
| (2) target root & wrapper bytes untouched (zero-write, wrapper-set digest) | — | `acceptance.TestScanCleanPasses` (zero-write proof), `norm.TestCleanFixturePasses` (wrapper-set digest via DigestSet) |
| (3) ordered list + every dvl record match the golden | — | `norm.TestCleanFixturePasses`, `norm.TestDVLFilterBoundary` |
| (4) no future snapshot/epoch record remains (harness-only mechanical apply, stats bit-identical) | — | `acceptance.TestMechanicalCleanEquality` (`assertNoFutureRecords` + stats bit-identity) |
| (5) next-epoch election equality over the masked view | reproduced-ss byte-equality (§4.6 out 3) | `acceptance.TestAuditTwoPassCloses` (`EpochTransition.ShardStateEqual`), `acceptance.TestElectionEqualityCleanVsMasked` (`committee.Compute` clean vs masked, encoded shard-state byte-equal) |
| (6) replacement-branch no stale-index reactivation | — | `acceptance.TestReplacementBranchNoReactivation` (real successor branch over the masked overlay; recreated dvl indexes carry only new `BlockNum`s), `norm.TestDVLFilterBoundary` |
| (9, scan half) corrupt records/state fail with the §4.5 codes | — | `acceptance.TestScanCorruptShardStateInvalid` (21), `acceptance.TestScanFallbackMissingSS`/`TestScanFallbackMissingBlkRwd` (20); `norm.TestInvalidListDecodeFatal`, `norm.TestDVLPointerMismatchFatal`, `norm.TestNoncanonicalSnapshotKeyFatal` |
| (7) apply crash/journal resume, second-apply zero mutations | **B4** | — |
| (8) refusal before head cut | **B4** | — |
| (9, head-cut half) | **B4** | — |
| two-donor convergence | superseded by junk-insensitivity + double-run determinism (§8 Q1) | `acceptance.TestJunkInsensitivity`, `acceptance.TestExportByteReproducible`, `acceptance.TestDeterminismSelfCheckCatchesFault` |

## Cross-cutting

| Concern | Test |
|---------|------|
| Exit-code precedence (mixed MISSING+INVALID → 21) | `report.TestResolveExitPrecedence`, `report.TestExitForFindings`, `norm.TestMixedExitPrecedence` |
| Canonical JSON stability / no HTML-escape | `report.TestCanonicalJSONStableAndSorted`, `report.TestCanonicalJSONNumbersVerbatim` |
| Strict RO opener regressions (missing LOCK, concurrent writer, corrupt manifest no-recovery, refusing methods) | `dbopen.TestMissingLockCreatesNothing`, `TestConcurrentWriterRefused`, `TestCorruptManifestNoRecovery`, `TestStrictStorageMethodsRefuse`, `TestLockNeverCreates`, `TestErrorIfMissing`, `TestValidateOutputPathRejectsInsideDB`, `TestCheckLayout` |
| strictdb namespace classifier + iterator error latching | `strictdb.TestClassifyNamespaces`, `TestClassifyNoncanonicalEpochSuffix`, `TestForEachChecksIteratorError` |
| Anchor config round-trip + rejections + drift pin | `anchor.TestResolveLocalnetRoundTrip`, `TestResolveRejects*`, `TestMainnetDriftPin`, `cmd/harmony-recovery.TestAnchorDriftPin` |
| HMR1 golden + decoder rejections + fuzz | `hmr.TestEncodeGoldenVector`, `TestDecoderRejections`, `FuzzDecode`, `TestEncodeDecodeRoundTrip`, `TestPackageDigestChangesOnFlip` |
| Reference manifest: excludes planned_deletions/timestamps, digest bindings, junk-insensitive, exact section cardinalities + canonical assertion set/order with zero end-state | `hmr.TestManifestExcludesPlannedDeletionsAndTimestamps`, `TestReferenceDigestBindings`, `TestManifestRoundTrip`, `TestManifestRejectsUnknownFields`, `TestManifestValidationRejectsIncomplete` |
| Integrity: seal exactly-four, stray fails, regenerate no-stale, verify | `integrity.TestSealExactFilesOnly`, `TestSealRegeneratesNoStaleEntry`, `TestWriteVerifyRoundTrip`, `TestFormatIsSha256sumCompatible` |
| Release sealing over real artifacts + anchor_config_sha256 match | `acceptance.TestReleaseSealing` |
| Pointer invariant solver (branch-advance, ambiguity+trusted, unique, replay validation) | `audit.TestSolverBranchAdvance`, `TestSolverAmbiguous`, `TestSolverUniqueSolution`, `TestSolverTrustedValidatesBranchReplay` |
| Masked overlay (mask/seed/barrier, merged ordered iteration, source immutability) | `audit.TestOverlayMaskAndSeed`, `TestOverlayMergedIterationOrdered` |
| Audit two-pass over real branch (roots match, reconciliation closes, staking + 0xfc precompile matrix reconciled, epoch-transition byte-equal, throughput floor) | `acceptance.TestAuditTwoPassCloses` |
| 0xfc precompile delegations (direct / nested contract→0xfc / reverted / top-up) classified and reconciled | `acceptance.TestAuditTwoPassCloses` (per-event assertions over the fixture matrix at blocks 45–48) |
| Injected-fault matrix: tampered seal → validity anomaly (24); tampered header root → fatal; tampered embedded LastCommitSignature → `verify-header` finding (24); planted future metadata key → plan-key anomaly (24); suppressed reproduction of an otherwise-expected branch rewrite (planted branch-height dvl append no branch block carries) → `plan-key-not-reproduced` (24) while the real branch rewrites still reproduce | `acceptance.TestAuditTamperedSealFinding`, `TestAuditRootMismatchFatal`, `TestAuditTamperedHeaderVerifyHeaderFinding`, `TestAuditPlantedKeyAnomaly`, `TestAuditSuppressedBranchRewriteAnomaly` |
| Known-bad cross-check excuses ONLY the incoming-receipts exploit failure (gate satisfied ⇔ receipt failure at the first known-bad height; absent → anomaly; wrong failure → anomaly; a collateral seal/VRF/header failure at the SAME height → `known-bad-extra-failure`, exit 24) | `audit.TestCrossCheckKnownBad` (exhaustive unit drive of the pure gate incl. the receipt-only exit-0 path), `acceptance.TestAuditKnownBadGate` (absent), `TestAuditKnownBadExtraFailureAnomalous` (receipt failure reproduced but the collateral header-signature defect still gates), `TestAuditKnownBadWrongFailure` (seal failure does not satisfy) |
| Native EditValidator + Undelegate and 0xfc precompile Undelegate inventoried and reconciled on an isolated fixture | `acceptance.TestAuditNativeDirectiveMatrix` |
| Native CollectRewards + 0xfc precompile CollectRewards through the real audit loop (validator elected at epoch 3, localnet aggregated payout at block 47 funds both pre-snapshot delegations; branch collects natively and via 0xfc; both inventoried, reconciliation closes, exit 0) | `acceptance.TestAuditCollectRewards` |
| Reward-cache process isolation: `internal/chain`'s payout caches are keyed only by (epoch, address), not chain identity, so two same-process fixtures with different payout-epoch snapshots cross-poison each other's block-47 payout (production `harmony-recovery` opens one chain per process and is unaffected; fixing the caches/`AddReward` is out-of-scope consensus code) — the divergent-snapshot test re-executes itself in a fresh subprocess | `acceptance.runIsolatedSubtest` (mechanism), applied by `TestAuditCollectRewards`; rationale in `acceptance/isolation_test.go` |
| Shard-1 crosslink subset + pass-two pollution clearing + pointer solver through the real audit loop (genuine committee-signed shard-1 crosslinks pre/post target; pass-1 `errAlreadyExist` pollution → masked in pass 2 → `VerifyCrossLink` passes; pre-target pointer derived uniquely; exit 0) | `acceptance.TestAuditShard1CrossLinkPollution` |
| Shard-1 spent-marker subset + pass-two pollution clearing through the real audit loop (genuine incoming cross-shard receipt applied at proposal, so the branch re-executes to its roots; pass-1 double-spent pollution → masked in pass 2 → full `ValidateCXReceiptsProof` passes; exit 0) | `acceptance.TestAuditSpentMarkerPollution` |
| Mutated stored `IncomingReceipts` with roots verifying (proof-material mutation leaves execution untouched: block re-executes to its stored root, proof verification fails → `unexpected-validity-failure`, exit 24) | `acceptance.TestAuditMutatedIncomingReceiptDetected`, `TestAuditLegacyReceiptBitmapUnrestorable` |
| Legacy `CXReceiptsProof.Copy` bitmap corruption (mainnet on-disk shape since 2019: stored `CommitBitmap` == `CommitSig`, produced NATIVELY by the fixture through the stock `WriteBlock` — a canary assertion pins the shape): the recovery-local repair restores the bitmap from the stored crosslink, proves it against the header incoming-receipt commitment, and verifies the full proof (exit 0, restoration inventoried); with the crosslink removed the corruption is unrestorable and fails closed (24) | `acceptance.TestAuditLegacyReceiptBitmapRestored`, `TestAuditLegacyReceiptBitmapUnrestorable` (restoration source removed) |
| Negative `--scratch-reserve-gb` refused at invocation (no production bypass) | `acceptance.TestAuditRejectsNegativeReserve` |
| Reference manifest cross-check bound into the audit report hash chain; anchor-mismatched AND content-forged references refused (15) | `acceptance.TestAuditReferenceCrossCheck` |
| Determinism self-check fault → exit 23, no artifacts, diff dumps | `acceptance.TestDeterminismSelfCheckCatchesFault` |
| Export publication is failure-atomic, one visible unit: artifacts, checksums AND the finalized success report are staged together, verified (exact names, none empty, any error fails) and published as `<out-dir>/release/` through ONE atomic directory rename — an incomplete set can never appear under the release name on any injected failure path (rename fault; verification fault; rename fault + cleanup fault stranding only non-consumer `.staging`) | `acceptance.TestExportPublishRenameFault`, `TestExportVerificationFault`, `TestExportPublishFaultWithCleanupFault`, run-once refusal in `TestExportRunOnceRefusesExisting`, success-report-in-release asserted in `TestExportByteReproducible` |
| A success-shaped export report is never visible without `release/` and vice versa: the success report publishes atomically inside `release/`; failed/refused attempts write a FAILURE report at the out-dir root; a staged-report write failure blocks the whole publication; even publish fault + failure-report fault leaves no success document anywhere consumer-visible | `acceptance.TestExportStagedReportWriteFault`, `TestExportFailureReportDoubleFault`, failure-report content asserted in `TestExportPublishRenameFault` |
| Source read errors AND malformed/incomplete canonical or header records surface as I/O/corruption (exit 14), never as absence: `side.Header` reads both keys through the strict adapter; a non-32-byte canonical mapping, a dangling canonical mapping (header record missing) and an undecodable header all fail closed everywhere; `(nil, nil)` is reserved for a genuinely absent canonical mapping; `CommitSigFor` never falls back to `block-sig-N` on any of these | `audit.TestSideHeaderCanonicalReadFault`, `TestSideHeaderRecordReadFault`, `TestSideHeaderUndecodable`, `TestSideHeaderGenuineAbsence`, `TestSideHeaderDanglingCanonicalMappingIsCorruption`, `TestSideHeaderMalformedCanonicalMapping`, `TestCommitSigForChildReadFaultNoFallback`, `TestCommitSigForGenuineAbsenceFallsBack`, `TestCommitSigForUsesChildHeader`, `TestPreconditionsChildReadFaultIsExitIO`, `TestPreconditionsChildAbsentIsBadInvocation` |
| Canonical/header/block identity mismatches (record decodes to a different hash/height than its mapping — tampering or a redirected mapping) can NEVER exit 0: `side.Header` and `side.Block` both validate identity; the precondition check turns the mismatch into exit 14; inside the pass the mismatch is recorded as a MANDATORY `source-identity` validity failure (gating to exit 24 through the known-bad cross-check, which never excuses it) while validation continues over the decoded content so ancestry/cryptographic/execution checks classify any accompanying tamper (Fatal root mismatch, verify-header/seal findings). The redirected-mapping fixture proves the pure-redirect case — every other check passes, only source-identity convicts | `audit.TestSideHeaderHashMismatchIsCorruption`, `TestSideHeaderHeightMismatchIsCorruption`, `TestSideBlockIdentityMismatch`; end-to-end: `acceptance.TestAuditRedirectedCanonicalMappingCannotExitZero` (exit 24 with roots all matched), tamper classification preserved in `TestAuditTamperedSealFinding`, `TestAuditRootMismatchFatal`, `TestAuditTamperedHeaderVerifyHeaderFinding`, `TestAuditKnownBadExtraFailureAnomalous` |
| CLI contract: help lists families, preflight preservation pin, no root-global flags, exit delivery | `cmd/harmony-recovery.TestRootHelpListsBothFamilies`, `TestMetadataHelpListsThreeSubcommands`, `TestPreflightPreservationPin`, `TestGoldenHelpFiles` |
| Dependency guard byte-unchanged through WS1–WS5; WS6 single exemption | `cmd/harmony-recovery.TestDependencyGuard` |
| Process isolation (no listeners / serve loop) | `cmd/harmony-recovery.TestMetadataProcessIsolation` |
| Generator reproducibility | `acceptance.TestGeneratorDeterministic`, `fixture.TestGenerateTwinChain` |
| Committed fixture kit pinned (fresh export byte-equals goldens, ground-truth digests, checksums verify) | `acceptance.TestCommittedKitGolden` over `testdata/recovery/metadata/kit` |
| Complete-tree kit reproducibility (every committed file — LevelDB dirs incl. `*.log`, `fixture-keys/*.hex`, anchor, goldens — regenerates byte-identical) | `acceptance.TestKitRegeneratesByteIdentical` via `fixture.GenerateKit` (same code path as the `gen` command) |

## Known local-fixture limitations (not blockers)

- A **receipt-ONLY validity failure at a known-bad height** (the exit-0
  gate-satisfied path) is not producible on a single-shard localnet fixture:
  tampering `IncomingReceiptHash` necessarily also breaks the header commit
  signature, and a body-level receipt tamper is applied by re-execution and
  diverges the state root into a FATAL insert. The gate-satisfied path is
  proven deterministically by the exhaustive pure-gate unit test
  (`audit.TestCrossCheckKnownBad`); the fixture path pins the collateral-
  failure behavior instead (`TestAuditKnownBadExtraFailureAnomalous`).
- The **mainnet pilot performance gate (§5.7)** is operator-devops owned; the
  runbook and report template live in `docs/recovery/metadata.md`.
