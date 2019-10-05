package discovery

import (
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// Service is the struct for discovery service.
type Service struct {
	host              p2p.Host
	peerChan          chan p2p.Peer
	stopChan          chan struct{}
	actionChan        chan nodeconfig.GroupAction
	config            service.NodeConfig
	actions           map[nodeconfig.GroupID]nodeconfig.ActionType
	messageChan       chan *msg_pb.Message
	addBeaconPeerFunc func(*p2p.Peer) bool
}

// New returns discovery service.
// h is the p2p host
// config is the node config
// (TODO: leo, build two overlays of network)
func New(h p2p.Host, config service.NodeConfig, peerChan chan p2p.Peer, addPeer func(*p2p.Peer) bool) *Service {
	return &Service{
		host:              h,
		peerChan:          peerChan,
		stopChan:          make(chan struct{}),
		actionChan:        make(chan nodeconfig.GroupAction),
		config:            config,
		actions:           make(map[nodeconfig.GroupID]nodeconfig.ActionType),
		addBeaconPeerFunc: addPeer,
	}
}

// StartService starts discovery service.
func (s *Service) StartService() {
	utils.Logger().Debug().Msg("Starting discovery service")
	s.Init()
	s.Run()
}

// StopService shutdowns discovery service.
func (s *Service) StopService() {
	utils.Logger().Debug().Msg("Shutting down discovery service")
	s.stopChan <- struct{}{}
	utils.Logger().Debug().Msg("discovery service stopped")
}

// NotifyService receives notification from service manager
func (s *Service) NotifyService(params map[string]interface{}) {
	data := params["peer"]
	action, ok := data.(nodeconfig.GroupAction)
	if !ok {
		utils.Logger().Error().Msg("Wrong data type passed to NotifyService")
		return
	}

	utils.Logger().Debug().Interface("got notified", action).Msg("[DISCOVERY]")
	s.actionChan <- action
}

// Run is the main function of the service
func (s *Service) Run() {
	go s.contactP2pPeers()
}

func (s *Service) contactP2pPeers() {
	pingInterval := 5

	nodeConfig := nodeconfig.GetShardConfig(s.config.ShardID)
	// Don't send ping message for Explorer Node
	if nodeConfig.Role() == nodeconfig.ExplorerNode {
		return
	}
	pingMsg := proto_discovery.NewPingMessage(s.host.GetSelfPeer(), s.config.IsClient)

	msgBuf := host.ConstructP2pMessage(byte(0), pingMsg.ConstructPingMessage())
	s.sentPingMessage(s.config.ShardGroupID, msgBuf)

	for {
		select {
		case peer, ok := <-s.peerChan:
			if !ok {
				utils.Logger().Debug().Msg("[DISCOVERY] No More Peer!")
				break
			}
			// TODO (leo) this one assumes all peers received in the channel are beacon chain node
			// It is just a temporary hack. When we work on re-sharding to regular shard, this has to be changed.
			if !s.config.IsBeacon {
				if s.addBeaconPeerFunc != nil {
					s.addBeaconPeerFunc(&peer)
				}
			}
		case <-s.stopChan:
			return
		}

		s.sentPingMessage(s.config.ShardGroupID, msgBuf)

		// the longest sleep is 3600 seconds
		if pingInterval >= 3600 {
			pingInterval = 3600
		} else {
			pingInterval *= 2
		}
		time.Sleep(time.Duration(pingInterval) * time.Second)
	}
}

// sentPingMessage sends a ping message to a pubsub topic
func (s *Service) sentPingMessage(g nodeconfig.GroupID, msgBuf []byte) {
	var err error
	// The following logical will be used for 2nd stage peer discovery process
	// do nothing when the groupID is unknown
	if s.config.ShardGroupID == nodeconfig.GroupIDUnknown {
		return
	}
	err = s.host.SendMessageToGroups([]nodeconfig.GroupID{s.config.ShardGroupID}, msgBuf)
	if err != nil {
		utils.Logger().Error().Err(err).Str("group", string(g)).Msg("Failed to send ping message")
	}
}

// Init is to initialize for discoveryService.
func (s *Service) Init() {
	utils.Logger().Debug().Msg("Init discovery service")
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
