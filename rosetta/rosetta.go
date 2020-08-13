package rosetta

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/harmony-one/harmony/hmy"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	common "github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rosetta/services"
)

// StartServers starts the rosetta http server
func StartServers(hmy *hmy.Harmony, config nodeconfig.RosettaServerConfig) error {
	if !config.HTTPEnabled {
		utils.Logger().Info().Msg("Rosetta http server disabled...")
		return nil
	}
	network := common.GetNetwork(hmy.ShardID)

	serverAsserter, err := asserter.NewServer(
		common.TransactionTypes,
		nodeconfig.GetDefaultConfig().Role() == nodeconfig.ExplorerNode,
		[]*types.NetworkIdentifier{network},
	)
	if err != nil {
		return err
	}

	router := server.CorsMiddleware(loggerMiddleware(getRouter(network, serverAsserter, hmy)))
	utils.Logger().Info().
		Int("port", config.HTTPPort).
		Str("ip", config.HTTPIp).
		Msg("Starting Rosetta server")

	endpoint := fmt.Sprintf("%s:%d", config.HTTPIp, config.HTTPPort)
	var (
		listener net.Listener
	)
	if listener, err = net.Listen("tcp", endpoint); err != nil {
		return err
	}
	go newHTTPServer(router).Serve(listener)
	return nil
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:      handler,
		ReadTimeout:  common.ReadTimeout,
		WriteTimeout: common.WriteTimeout,
		IdleTimeout:  common.IdleTimeout,
	}
}

func getRouter(
	network *types.NetworkIdentifier,
	asserter *asserter.Asserter,
	hmy *hmy.Harmony,
) http.Handler {
	return server.NewRouter(
		server.NewNetworkAPIController(services.NewNetworkAPIService(network, hmy), asserter),
		server.NewBlockAPIController(services.NewBlockAPIService(network, hmy), asserter),
	)
}

func loggerMiddleware(router http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		router.ServeHTTP(w, r)
		msg := fmt.Sprintf(
			"Rosetta: %s %s %s",
			r.Method,
			r.RequestURI,
			time.Since(start),
		)
		utils.Logger().Info().Msg(msg)
		// Print to stdout for quick check of rosetta activity
		fmt.Printf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	})
}
