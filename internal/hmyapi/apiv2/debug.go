package apiv2

import (
	"context"
	"errors"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/log"
)

// DebugAPI Internal JSON RPC for debugging purpose
type DebugAPI struct {
	b Backend
}

// NewDebugAPI Creates a new DebugAPI instance
func NewDebugAPI(b Backend) *DebugAPI {
	return &DebugAPI{b}
}

// SetLogVerbosity Sets log verbosity on runtime
// Example usage:
//  curl -H "Content-Type: application/json" -d '{"method":"debug_setLogVerbosity","params":[0],"id":1}' http://localhost:9123
func (*DebugAPI) SetLogVerbosity(ctx context.Context, level int) (map[string]interface{}, error) {
	if level < int(log.LvlCrit) || level > int(log.LvlTrace) {
		return nil, errors.New("invalid log level")
	}

	verbosity := log.Lvl(level)
	utils.SetLogVerbosity(verbosity)
	return map[string]interface{}{"verbosity": verbosity.String()}, nil
}
