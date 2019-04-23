package discovery

import (
	"time"

	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// Service is the struct for discovery service.
type Service struct {
	host              p2p.Host
	peerChan          chan p2p.Peer
	stopChan          chan struct{}
	actionChan        chan p2p.GroupAction
	config            service.NodeConfig
	actions           map[p2p.GroupID]p2p.ActionType
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
		actionChan:        make(chan p2p.GroupAction),
		config:            config,
		actions:           make(map[p2p.GroupID]p2p.ActionType),
		addBeaconPeerFunc: addPeer,
	}
}

// StartService starts discovery service.
func (s *Service) StartService() {
	utils.GetLogInstance().Info("Starting discovery service.")
	s.Init()
	s.Run()
}

// StopService shutdowns discovery service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Shutting down discovery service.")
	s.stopChan <- struct{}{}
	utils.GetLogInstance().Info("discovery service stopped.")
}

// NotifyService receives notification from service manager
func (s *Service) NotifyService(params map[string]interface{}) {
	data := params["peer"]
	action, ok := data.(p2p.GroupAction)
	if !ok {
		utils.GetLogInstance().Error("Wrong data type passed to NotifyService")
		return
	}

	utils.GetLogInstance().Info("[DISCOVERY]", "got notified", action)
	s.actionChan <- action
}

// Run is the main function of the service
func (s *Service) Run() {
	go s.contactP2pPeers()
}

func (s *Service) contactP2pPeers() {
	tick := time.NewTicker(5 * time.Second)

	pingMsg := proto_discovery.NewPingMessage(s.host.GetSelfPeer(), s.config.IsClient)

	utils.GetLogInstance().Error("Constructing Ping Message", "myPing", pingMsg)
	msgBuf := host.ConstructP2pMessage(byte(0), pingMsg.ConstructPingMessage())
	s.sentPingMessage(s.config.ShardGroupID, msgBuf)

	for {
		select {
		case peer, ok := <-s.peerChan:
			if !ok {
				utils.GetLogInstance().Debug("[DISCOVERY] No More Peer!")
				break
			}
			// TODO (leo) this one assumes all peers received in the channel are beacon chain node
			// It is just a temporary hack. When we work on re-sharding to regular shard, this has to be changed.
			if !s.config.IsBeacon {
				if s.addBeaconPeerFunc != nil {
					s.addBeaconPeerFunc(&peer)
				}
			}
			// Add to outgoing peer list
			// s.host.AddOutgoingPeer(peer)
			// utils.GetLogInstance().Debug("[DISCOVERY]", "add outgoing peer", peer)
		case <-s.stopChan:
			utils.GetLogInstance().Debug("[DISCOVERY] stop pinging ...")
			return
		case action := <-s.actionChan:
			s.config.Actions[action.Name] = action.Action
		case <-tick.C:
			for g, a := range s.config.Actions {
				if a == p2p.ActionPause {
					// Received Pause Message, to reduce the frequency of ping message to every 1 minute
					// TODO (leo) use different timer tick for different group, mainly differentiate beacon and regular shards
					// beacon ping could be less frequent than regular shard
					tick.Stop()
					tick = time.NewTicker(1 * time.Minute)
				}

				if a == p2p.ActionStart || a == p2p.ActionResume || a == p2p.ActionPause {
					s.sentPingMessage(g, msgBuf)
				}
			}
		}
	}
}

// sentPingMessage sends a ping message to a pubsub topic
func (s *Service) sentPingMessage(g p2p.GroupID, msgBuf []byte) {
	var err error
	if g == p2p.GroupIDBeacon || g == p2p.GroupIDBeaconClient {
		err = s.host.SendMessageToGroups([]p2p.GroupID{s.config.Beacon}, msgBuf)
	} else {
		// The following logical will be used for 2nd stage peer discovery process
		// do nothing when the groupID is unknown
		if s.config.ShardGroupID == p2p.GroupIDUnknown {
			return
		}
		err = s.host.SendMessageToGroups([]p2p.GroupID{s.config.ShardGroupID}, msgBuf)
	}
	if err != nil {
		utils.GetLogInstance().Error("Failed to send ping message", "group", g)
	}
}

// Init is to initialize for discoveryService.
func (s *Service) Init() {
	utils.GetLogInstance().Info("Init discovery service")
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}
