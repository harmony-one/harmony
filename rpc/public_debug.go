package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/utils"
)

// PublicDebugService Internal JSON RPC for debugging purpose
type PublicDebugService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicDebugAPI creates a new API for the RPC interface
// TODO(dm): expose public via config
func NewPublicDebugAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicDebugService{hmy, version},
		Public:    false,
	}
}

// SetLogVerbosity Sets log verbosity on runtime
func (s *PublicDebugService) SetLogVerbosity(ctx context.Context, level int) (map[string]interface{}, error) {
	if level < int(log.LvlCrit) || level > int(log.LvlTrace) {
		return nil, ErrInvalidLogLevel
	}

	verbosity := log.Lvl(level)
	utils.SetLogVerbosity(verbosity)
	return map[string]interface{}{"verbosity": verbosity.String()}, nil
}
