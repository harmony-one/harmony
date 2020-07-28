package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/utils"
)

// PrivateDebugService Internal JSON RPC for debugging purpose
type PrivateDebugService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPrivateDebugAPI creates a new API for the RPC interface
// TODO(dm): expose public via config
func NewPrivateDebugAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PrivateDebugService{hmy, version},
		Public:    false,
	}
}

// SetLogVerbosity Sets log verbosity on runtime
// Example usage:
//  curl -H "Content-Type: application/json" -d '{"method":"debug_setLogVerbosity","params":[0],"id":1}' http://localhost:9123
func (*PrivateDebugService) SetLogVerbosity(ctx context.Context, level int) (map[string]interface{}, error) {
	if level < int(log.LvlCrit) || level > int(log.LvlTrace) {
		return nil, ErrInvalidLogLevel
	}

	verbosity := log.Lvl(level)
	utils.SetLogVerbosity(verbosity)
	return map[string]interface{}{"verbosity": verbosity.String()}, nil
}
