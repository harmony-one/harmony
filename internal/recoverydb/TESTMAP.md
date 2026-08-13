# harmony-recovery-db test map

Every retained consensus-critical acceptance bullet from the clean-DB handoff
§17.1/§17.2 and the plan (WS8) maps to a concrete test here. Bullets that
existed only for descoped machinery (two-donor-mandatory, compact-input
normalization, full-profile reconciliation, the validator installer, the
broad packaging attack matrix) are marked **descoped** with the §9 pointer —
not silently dropped.

Run the whole suite (macOS needs the BLS dylib on the loader path):

```sh
source scripts/setup_bls_build_flags.sh
go test -exec "/usr/bin/env DYLD_FALLBACK_LIBRARY_PATH=$LD_LIBRARY_PATH" ./internal/recoverydb/... ./cmd/harmony-recovery-db/
# or:
make test-recovery
```

## WS1 — foundation

| Requirement | Test |
| --- | --- |
| anchor round-trip + rejection vectors incl. pinned target hash/parent mismatch | `anchor` `TestLoadRoundTrip`, `TestRejectionVectors` |
| `Window(MainnetSchedule, 92730034) == [92700671, 92730034]`, 29364 blocks | `anchor` `TestWindowPinnedValues` |
| property `CalcEpochNumber(EpochLastBlock(e)) == e` across TwoSeconds (mainnet+localnet) | `anchor` `TestEpochLastBlockProperty` |
| strict-opener corruption: corrupted MANIFEST / truncated SST, directory fingerprint unchanged, no recovery | `dbopen` `TestStrictOpenerCorruption` |
| stock geth wrapper demonstrably mutates the same fixture (regression guard) | `dbopen` `TestStrictOpenerCorruption/stockWrapperMutates` |
| read-only adapter refuses every write-shaped method; batch `ValueSize`/`Reset`/`Replay`; interface assertion | `dbopen` `TestMutationRefusal` (compile-time `var _ ethdb.KeyValueStore`) |
| dbopen refuses live-held dir, sharded layout, relative path | `dbopen` `TestRefusals` |
| read-only opens take compatible shared locks; writable open fails while held | `dbopen` `TestReadOnlyIsSharedLock` |
| strictdb latches injected batch errors; partial batch never commits | `strictdb` `TestLatchingBatch` |
| strictdb ForEach surfaces iterator/callback errors; write-refusing decorator | `strictdb` `TestForEachSurfacesIteratorError`, `TestWriteRefusing` |
| integrity round-trips checksum files, detects single-byte corruption, chain links | `integrity` `TestChecksumFileRoundTrip`, `TestInputRefChain`, `TestSums` |
| journal transitions incl. reopen-refusal of IN_PROGRESS; substates; COMPLETE_UNRELEASABLE | `report` `TestJournalStateMachine` |
| durability fsync-walk covered by injected-failure fs wrapper | `report` `TestFsyncWalkInjectedFailure` |
| digest determinism, domain separation, length-prefixing, DigestSet validate/diff | `report` `TestDigestDeterminismAndDomains`, `TestDigestSetValidateAndDiff` |
| raw key-schema pinned against stock rawdb accessors | `keys` `TestSchemaPinning` |

## WS2 — inspect-db / inventory-db

| Requirement | Test |
| --- | --- |
| both copies → byte-identical baseline tuples + equal DigestSets via agreement | `e2e` `TestPipelineEndToEnd` (inspect A/B + agreement) |
| deleted `c`/`vc`/legacy code fixtures fail naming the hash | `verify` `TestDeletedCodeFatal` |
| stock-iterator-defect regression: deleted storage-root node fatal (defect 1) | `verify` `TestDeletedStorageNodeFatal` |
| validator-code-only + legacy fallback traverse via the purpose-built path (defect 2) | `verify` `TestCodeFallbackLocations` |
| wrong-content code fatal (stock iterator accepts it — comparison assertion) | `verify` `TestWrongContentCodeFatal` |
| deleted account-/storage-slot-preimage fixtures fail under `--require-preimages`, counted without | `verify` `TestPreimageCoverage` |
| classifier collision basics: `cl`/`cx*` never code; legacy code + orphan node → bare-hash32; malformed itemized | `keys` `TestClassifier` |
| preflight refuses a stock-dumpdb-shaped fixture (SnapdbInfo / missing LastFast) | `inspect` (WS2 preflight `refuse` paths); exercised via full-archival gate in `e2e` |

## WS3 — export-bundle (+ compare-bundles)

| Requirement | Test |
| --- | --- |
| round-trip decode of every chunk; multi-chunk layout | `e2e` `TestPipelineEndToEnd` (ChunkBytes forced small) |
| donor gap mid-range → precise block-accurate refusal | `e2e` `TestExportFaults/donorGapMidRange` |
| wrong `--from-height` vs baseline manifest refuses | `e2e` `TestExportFaults/wrongFromHeight` |
| two exports of the same donor are byte-identical | `e2e` `TestExportFaults/deterministicChunks` |
| certificate per exported record verifies under the donor committee | export path in `e2e` (BLS verify inside `bundle.Export`) |
| sidecar parent == pinned target hash; header hash == ABANDONED_CHILD_HASH | `bundle.Export` sidecar assertions (`e2e`) |
| `compare-bundles` passes on identical chains | `e2e` `TestExportFaults/compareBundles` |
| **descoped**: two-donor-mandatory byte-equality | §9 revision 6 (single-donor is the supported mode) |

## WS4 — replay-bundle

| Requirement | Test |
| --- | --- |
| localnet E2E replays to exactly the target tuple, gate all-green, rerun refuses | `e2e` `TestPipelineEndToEnd` |
| corrupted checksum (anchor / chunk / truncated chunk) | `e2e` `TestReplayFaults/{corruptedAnchorChecksum,corruptedChunk,truncatedChunk}` |
| bundle extending past the target rejected outright | `e2e` `TestReplayFaults/bundlePastTarget` |
| planted pre-existing block at a bundle height (ErrKnownBlock semantics) | `e2e` `TestReplayFaults/plantedPreexistingBlock` |
| wrong parent | `e2e` `TestReplayFaults/wrongParentRecord` |
| wrong certificate (two valid aggregates, wrong block) | `e2e` `TestReplayFaults/wrongCertificate` |
| tampered record bytes | `e2e` `TestReplayFaults/tamperedBodyBytes` |
| invalid VRF, re-signed → rejected by ValidateNewBlock (two-layer defense) | `e2e` `TestReplayFaults/invalidVRFResigned` |
| wrong execution/state root, re-signed → rejected by ValidateNewBlock | `e2e` `TestReplayFaults/wrongStateRootResigned` |
| sidecar hash ≠ ABANDONED_CHILD_HASH | `e2e` `TestReplayFaults/sidecarAnchorMismatch` |
| SIGKILL mid-mutation ⇒ IN_PROGRESS, reopen refused, fresh copy replays clean | `e2e` `TestReplaySIGKILL` (+ `TestReplayCrashHelper` child) |
| pending-queue clear recorded; markers deleted/itemized (in-place §2.2 alignment) | replay finalizer + gate in `e2e` `TestPipelineEndToEnd`; absence asserted by `verify` `TestVerifySeededDefects/plantedPendingQueue` |

## WS5 — compact-db

| Requirement | Test |
| --- | --- |
| source-not-at-target / nonempty destination / read-only refusals | argument + gate checks (`compact.Run`), CLI in `cmd` `TestCLIContract` |
| supplied manifest failing digest / diverged sections refuse with NO head key written | `e2e` `TestReferenceMode/divergedMetadataRefusesBeforeHeads` |
| absent manifest builds cleanly with `internal:none` sentinel, mode recorded | `e2e` `TestPipelineEndToEnd` (internal mode) |
| every retained block has a verifying exact `block-sig-N`; window boundary present | `verify` window-cert check via `e2e` |
| two independent builds produce identical logical KV digests | `e2e` `TestTwoBuildDigestEquality` |
| offline reopen reaches exactly the target tuple (all four heads + marker) | `e2e` `TestPipelineEndToEnd` (installed-payload reopen) |
| oversized-but-complete build lands COMPLETE_UNRELEASABLE, refused by package-db | `e2e` `TestPackageCrashWindows/refusesUnreleasableBuild` (journal state) |
| **descoped**: compact-input normalization (SnapdbInfo/LastFast synthesis) | §9 revision 6 (full-archival input only) |

## WS6 — verify-db (seeded-defect matrix; unmodified fixture passes)

| Seeded defect | Test | Check ID |
| --- | --- | --- |
| stale fallback-only cert (LastCommits, exact key deleted) | `TestVerifySeededDefects/staleFallbackOnlyCert` | `raw.window.certs` |
| missing inverse mapping at target | `.../missingInverseMapping` | `raw.canonical.target` |
| planted future lookup | `.../plantedFutureLookup` | `raw.window.lookups` |
| planted abandoned-child header-number entry | `.../plantedAbandonedChildEntry` | `raw.known-bad.abandoned-child` |
| stale `LastPivot` | `.../stalePivot` | `raw.runtime-markers` |
| forged CX-spent entry | `.../forgedCXSpent` | `digestset.match` |
| planted pending-queue key | `.../plantedPendingQueue` | `raw.pending-queues` |
| planted validator-stats key (no opt-in) | `.../plantedValidatorStats` | `raw.validator-stats` |
| epoch+2 shard state (beyond epoch-last allowance) | `.../epochPlusOneShardState` | `raw.above-target` |
| fork block (header at legal height, non-canonical hash) | `.../forkBlock` | `raw.forks` |
| corrupted mid-window `block-sig-N` | `.../corruptedMidWindowSig` | `raw.window.certs` |
| planted unresolved bare-hash32 key ⇒ fatal | `.../unresolvedBareHash32Fatal` | `raw.bare-hash32` |
| orphan prefixed code key ⇒ fatal | `.../orphanPrefixedCode` | `raw.code-orphans` |
| wrong heads (LastFinalized ≠ target) | `.../wrongHeads` | `raw.heads` |
| missing recovery marker | `.../missingMarker` | `raw.marker.present` |
| marker wrong metadata-reference digest | `.../markerWrongReference` | `raw.marker.reference-mode` |
| marker self-referential logical digest | `.../markerSelfReferenceDigest` | `raw.marker.logical-digest` |
| marker wrong tool version / binary SHA (exact-equality) | `.../markerWrongToolVersion`, `.../markerWrongBinarySHA` | `raw.marker.tool-identity` |
| mode mismatch (manifest supplied to an internal-mode build) | `.../modeMismatchManifestSupplied` | `raw.marker.reference-mode` |
| reference-mode build verified without manifest (mode mismatch) | `TestReferenceMode/buildAndVerify` | `raw.marker.reference-mode` |
| diverged-metadata reference manifest (per-section convergence) | `TestReferenceMode/divergedMetadataRefusesBeforeHeads` (compact side) | — |
| digest determinism across a clean reopen re-traversal | `verify.go` §11.2 reopen + `cmd` verify | `state.reopen` |
| logical digest marker-exclusion (present == deleted; operator-decided three-key exclusion set) | `verify` `TestLogicalDigestMarkerExclusion` | `logical.digest-match` |
| preimage marker pair contract (complete pair or neither; exact pinned values) | `verify` `TestValidatePreimageMarkers` | `raw.runtime-markers` |
| half preimage pair (one half deleted) ⇒ fatal | `.../halfPreimagePairStartOnly`, `.../halfPreimagePairEndOnly` | `raw.runtime-markers` |

## WS7 — package-db (checksum-mismatch basics + journaled seal)

| Requirement | Test |
| --- | --- |
| stage → re-verify → seal; SHA256SUMS covers release.json + INSTALL.md, excludes {itself, READY}; non-circular | `e2e` `TestPipelineEndToEnd` (sums assertions) |
| kill between rename and READY ⇒ rerun re-verifies unsealed dir, seals, completes | `e2e` `TestPackageCrashWindows/killBetweenRenameAndREADY` |
| kill between READY and terminal record ⇒ rerun re-verifies sealed dir, completes | `e2e` `TestPackageCrashWindows/killBetweenREADYAndTerminal` |
| tampered promoted-but-unsealed tree ⇒ quarantined | `e2e` `TestPackageCrashWindows/tamperedSealedTreeQuarantined` |
| unjournaled final directory ⇒ quarantined | `e2e` `TestPackageCrashWindows/unjournaledDirQuarantined` |
| refuses a non-COMPLETE_VERIFIED / UNRELEASABLE build | `e2e` `TestPackageCrashWindows/refusesUnreleasableBuild` |
| byte-identical rebuild against a sealed release refused as already-existing | `e2e` `TestPipelineEndToEnd` (rebuild refusal) |
| **descoped**: `files.jsonl` payload manifest + broad attack matrix | §9 revision 14 (went with the installer) |

## WS8 — cross-cutting

| Requirement | Test |
| --- | --- |
| CLI required/forbidden flag matrix incl. both reference-manifest modes | `cmd` `TestCLIContract` |
| localnet fixture kit with real BLS certificates (replay-grade by construction) | `fixture` `TestGenerateSmoke` |
| full localnet E2E: inspect→export→replay→compact→verify→package→install→offline reopen | `e2e` `TestPipelineEndToEnd`; `scripts/recovery/e2e-localnet.sh` (stock-binary boot smoke) |
| epoch-boundary crossing during replay (baseline 18 → target 22 crosses epoch 1→2) | `e2e` `TestPipelineEndToEnd` (window spans epochs) |
| **descoped**: installer tests, ceremony criteria, full-profile reconciliation | §9 revisions 6/11/14 |
| **adoption-time**: `v2026.1.2-recovery` boot rehearsal | §8 integration item (in-place agents; not a CI gate) |
