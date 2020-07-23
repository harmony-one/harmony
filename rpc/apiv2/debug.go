package apiv2

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/utils"
)

// DebugAPI Internal JSON RPC for debugging purpose
type DebugAPI struct {
	hmy *hmy.Harmony
}

// NewDebugAPI Creates a new DebugAPI instance
func NewDebugAPI(hmy *hmy.Harmony) *DebugAPI {
	return &DebugAPI{hmy}
}

// SetLogVerbosity Sets log verbosity on runtime
// Example usage:
//  curl -H "Content-Type: application/json" -d '{"method":"debug_setLogVerbosity","params":[0],"id":1}' http://localhost:9123
func (*DebugAPI) SetLogVerbosity(ctx context.Context, level int) (map[string]interface{}, error) {
	if level < int(log.LvlCrit) || level > int(log.LvlTrace) {
		return nil, ErrInvalidLogLevel
	}

	verbosity := log.Lvl(level)
	utils.SetLogVerbosity(verbosity)
	return map[string]interface{}{"verbosity": verbosity.String()}, nil
}
