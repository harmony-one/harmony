package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2pv2"
	"github.com/harmony-one/harmony/proto"
	"github.com/harmony-one/harmony/proto/client"
	"github.com/harmony-one/harmony/proto/consensus"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
	proto_node "github.com/harmony-one/harmony/proto/node"
	netp2p "github.com/libp2p/go-libp2p-net"
)

// NodeHandlerV1 handles a new incoming connection.
func (node *Node) NodeHandlerV1(s netp2p.Stream) {
	defer s.Close()

	// Read p2p message payload
	content, err := p2pv2.ReadData(s)

	if err != nil {
		node.log.Error("Read p2p data failed", "err", err, "node", node)
		return
	}
	// TODO: this is tree broadcasting. this needs to be removed later. Actually the whole logic needs to be replaced by p2p.
	node.MaybeBroadcastAsValidator(content)

	consensusObj := node.Consensus

	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		node.log.Error("Read node type failed", "err", err, "node", node)
		return
	}

	msgType, err := proto.GetMessageType(content)
	if err != nil {
		node.log.Error("Read action type failed", "err", err, "node", node)
		return
	}

	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		node.log.Error("Read message payload failed", "err", err, "node", node)
		return
	}

	switch msgCategory {
	case proto.Identity:
		actionType := proto_identity.IDMessageType(msgType)
		switch actionType {
		case proto_identity.Identity:
			messageType := proto_identity.MessageType(msgPayload[0])
			switch messageType {
			case proto_identity.Register:
				fmt.Println("received a identity message")
				// TODO(ak): fix it.
				// node.processPOWMessage(msgPayload)
				node.log.Info("NET: received message: IDENTITY/REGISTER")
			default:
				node.log.Error("Announce message should be sent to IdentityChain")
			}
		}
	case proto.Consensus:
		actionType := consensus.ConMessageType(msgType)
		switch actionType {
		case consensus.Consensus:
			if consensusObj.IsLeader {
				node.log.Info("NET: received message: Consensus/Leader")
				consensusObj.ProcessMessageLeader(msgPayload)
			} else {
				node.log.Info("NET: received message: Consensus/Validator")
				consensusObj.ProcessMessageValidator(msgPayload)
			}
		}
	case proto.Node:
		actionType := proto_node.MessageType(msgType)
		switch actionType {
		case proto_node.Transaction:
			node.log.Info("NET: received message: Node/Transaction")
			node.transactionMessageHandler(msgPayload)
		case proto_node.Block:
			node.log.Info("NET: received message: Node/Block")
			blockMsgType := proto_node.BlockMessageType(msgPayload[0])
			switch blockMsgType {
			case proto_node.Sync:
				decoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the Sync messge type
				blocks := new([]*blockchain.Block)
				decoder.Decode(blocks)
				if node.Client != nil && node.Client.UpdateBlocks != nil && blocks != nil {
					node.Client.UpdateBlocks(*blocks)
				}
			}
		case proto_node.Client:
			node.log.Info("NET: received message: Node/Client")
			clientMsgType := proto_node.ClientMessageType(msgPayload[0])
			switch clientMsgType {
			case proto_node.LookupUtxo:
				decoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the LookupUtxo messge type

				fetchUtxoMessage := new(proto_node.FetchUtxoMessage)
				decoder.Decode(fetchUtxoMessage)

				utxoMap := node.UtxoPool.GetUtxoMapByAddresses(fetchUtxoMessage.Addresses)

				p2p.SendMessage(fetchUtxoMessage.Sender, client.ConstructFetchUtxoResponseMessage(&utxoMap, node.UtxoPool.ShardID))
			}
		case proto_node.Control:
			node.log.Info("NET: received message: Node/Control")
			controlType := msgPayload[0]
			if proto_node.ControlMessageType(controlType) == proto_node.STOP {
				if node.Chain == nil {
					node.log.Debug("Stopping Node", "node", node, "numBlocks", len(node.blockchain.Blocks), "numTxsProcessed", node.countNumTransactionsInBlockchain())

					sizeInBytes := node.UtxoPool.GetSizeInByteOfUtxoMap()
					node.log.Debug("UtxoPool Report", "numEntries", len(node.UtxoPool.UtxoMap), "sizeInBytes", sizeInBytes)

					avgBlockSizeInBytes := 0
					txCount := 0
					blockCount := 0
					totalTxCount := 0
					totalBlockCount := 0
					avgTxSize := 0

					for _, block := range node.blockchain.Blocks {
						if block.IsStateBlock() {
							totalTxCount += int(block.State.NumTransactions)
							totalBlockCount += int(block.State.NumBlocks)
						} else {
							byteBuffer := bytes.NewBuffer([]byte{})
							encoder := gob.NewEncoder(byteBuffer)
							encoder.Encode(block)
							avgBlockSizeInBytes += len(byteBuffer.Bytes())

							txCount += len(block.Transactions)
							blockCount++
							totalTxCount += len(block.TransactionIds)
							totalBlockCount++

							byteBuffer = bytes.NewBuffer([]byte{})
							encoder = gob.NewEncoder(byteBuffer)
							encoder.Encode(block.Transactions)
							avgTxSize += len(byteBuffer.Bytes())
						}
					}
					if blockCount != 0 {
						avgBlockSizeInBytes = avgBlockSizeInBytes / blockCount
						avgTxSize = avgTxSize / txCount
					}

					node.log.Debug("Blockchain Report", "totalNumBlocks", totalBlockCount, "avgBlockSizeInCurrentEpoch", avgBlockSizeInBytes, "totalNumTxs", totalTxCount, "avgTxSzieInCurrentEpoch", avgTxSize)
				} else {
					node.log.Debug("Stopping Node (Account Model)", "node", node, "CurBlockNum", node.Chain.CurrentHeader().Number, "numTxsProcessed", node.countNumTransactionsInBlockchainAccount())
				}

				os.Exit(0)
			}
		case proto_node.PING:
			// Leader receives PING from new node.
			node.pingMessageHandler(msgPayload)
		case proto_node.PONG:
			// The new node receives back from leader.
			node.pongMessageHandler(msgPayload)
		}
	case proto.Client:
		actionType := client.MessageType(msgType)
		node.log.Info("NET: received message: Client/Transaction")
		switch actionType {
		case client.Transaction:
			if node.Client != nil {
				node.Client.TransactionMessageHandler(msgPayload)
			}
		}
	default:
		node.log.Error("Unknown", "MsgCateory:", msgCategory)
	}
}
