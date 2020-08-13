package rosetta

import (
	"fmt"
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
func StartServers(hmy *hmy.Harmony, port int) error {
	network := common.GetNetwork(hmy.ShardID)
	isExplorer := nodeconfig.GetDefaultConfig().Role() == nodeconfig.ExplorerNode

	serverAsserter, err := asserter.NewServer(
		common.TransactionTypes,
		isExplorer,
		[]*types.NetworkIdentifier{network},
	)
	if err != nil {
		return err
	}

	router := getRouter(network, serverAsserter, hmy)
	loggedRouter := loggerMiddleware(router)
	corsRouter := server.CorsMiddleware(loggedRouter)
	utils.Logger().Info().Int("port", port).Msg("Starting Rosetta server")
	utils.Logger().Err(http.ListenAndServe(fmt.Sprintf(":%d", port), corsRouter))
	return nil
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
		fmt.Printf(msg) // Print to stdout for quick glace at rosetta activity
	})
}
