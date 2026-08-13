package report

import "os"

// CrashPointEnv names the crash point the process must die at, for the
// deterministic crash matrix of the acceptance tests (plan WS8; round 13
// finding 10). Unset in production, where every CrashPoint call is inert.
const CrashPointEnv = "RECOVERYDB_CRASHPOINT"

// CrashPoint kills the process immediately (no deferred cleanup, no journal
// completion, no report writing — the same observable effect as SIGKILL at
// this instruction) when name matches $RECOVERYDB_CRASHPOINT.
func CrashPoint(name string) {
	if os.Getenv(CrashPointEnv) == name {
		os.Exit(137)
	}
}
