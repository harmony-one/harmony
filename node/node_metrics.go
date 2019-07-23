package node

import (
	"time"

	"github.com/harmony-one/harmony/internal/utils"
)

// Collect and store metrics into metrics ldb
func (node *Node) CollectMetrics() {
	utils.Logger().Info().Msg("Init metrics db.")
	node.metricsStorage = utils.GetMetricsStorageInstance(node.ClientPeer.IP, node.ClientPeer.Port, true)
	// flush peers number each second
    for _ = range time.Tick(1000 * time.Millisecond) {
    	node.metricsStorage.Dump(node.numPeers, int(time.Now().Unix()))
   	}
}