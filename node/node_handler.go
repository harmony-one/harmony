package node

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto"
	"github.com/simple-rules/harmony-benchmark/proto/client"
	"github.com/simple-rules/harmony-benchmark/proto/consensus"
	proto_identity "github.com/simple-rules/harmony-benchmark/proto/identity"
	proto_node "github.com/simple-rules/harmony-benchmark/proto/node"
)

const (
	// The max number of transaction per a block.
	MaxNumberOfTransactionsPerBlock = 10000
	// The number of blocks allowed before generating state block
	NumBlocksBeforeStateBlock = 1000
)

func (node *Node) MaybeBroadcastAsValidator(content []byte) {
	if node.SelfPeer.ValidatorID > 0 && node.SelfPeer.ValidatorID <= p2p.MAX_BROADCAST {
		go p2p.BroadcastMessageFromValidator(node.SelfPeer, node.Consensus.GetValidatorPeers(), content)
	}
}

// NodeHandler handles a new incoming connection.
func (node *Node) NodeHandler(conn net.Conn) {
	defer conn.Close()

	// Read p2p message payload
	content, err := p2p.ReadMessageContent(conn)

	if err != nil {
		node.log.Error("Read p2p data failed", "err", err, "node", node)
		return
	}
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
	case proto.IDENTITY:
		actionType := proto_identity.IdentityMessageType(msgType)
		switch actionType {
		case proto_identity.IDENTITY:
			messageType := proto_identity.MessageType(msgPayload[0])
			switch messageType {
			case proto_identity.REGISTER:
				fmt.Println("received a identity message")
				node.processPOWMessage(msgPayload)
			case proto_identity.ANNOUNCE:
				node.log.Error("Announce message should be sent to IdentityChain")
			}
		}
	case proto.CONSENSUS:
		actionType := consensus.ConsensusMessageType(msgType)
		switch actionType {
		case consensus.CONSENSUS:
			if consensusObj.IsLeader {
				consensusObj.ProcessMessageLeader(msgPayload)
			} else {
				consensusObj.ProcessMessageValidator(msgPayload)
			}
		}
	case proto.NODE:
		actionType := proto_node.NodeMessageType(msgType)
		switch actionType {
		case proto_node.TRANSACTION:
			node.transactionMessageHandler(msgPayload)
		case proto_node.BLOCK:
			blockMsgType := proto_node.BlockMessageType(msgPayload[0])
			switch blockMsgType {
			case proto_node.SYNC:
				decoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the SYNC messge type
				blocks := new([]*blockchain.Block)
				decoder.Decode(blocks)
				if node.Client != nil && node.Client.UpdateBlocks != nil && blocks != nil {
					node.Client.UpdateBlocks(*blocks)
				}
			}
		case proto_node.BLOCKCHAIN_SYNC:
			node.handleBlockchainSync(msgPayload, conn)
		case proto_node.CLIENT:
			clientMsgType := proto_node.ClientMessageType(msgPayload[0])
			switch clientMsgType {
			case proto_node.LOOKUP_UTXO:
				decoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the LOOKUP_UTXO messge type

				fetchUtxoMessage := new(proto_node.FetchUtxoMessage)
				decoder.Decode(fetchUtxoMessage)

				utxoMap := node.UtxoPool.GetUtxoMapByAddresses(fetchUtxoMessage.Addresses)

				p2p.SendMessage(fetchUtxoMessage.Sender, client.ConstructFetchUtxoResponseMessage(&utxoMap, node.UtxoPool.ShardID))
			}
		case proto_node.CONTROL:
			controlType := msgPayload[0]
			if proto_node.ControlMessageType(controlType) == proto_node.STOP {
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
						blockCount += 1
						totalTxCount += len(block.TransactionIds)
						totalBlockCount += 1

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

				os.Exit(0)
			}
		}
	case proto.CLIENT:
		actionType := client.ClientMessageType(msgType)
		switch actionType {
		case client.TRANSACTION:
			if node.Client != nil {
				node.Client.TransactionMessageHandler(msgPayload)
			}
		}
	}
}

// Refactor by moving this code into a sync package.
func (node *Node) handleBlockchainSync(payload []byte, conn net.Conn) {
	// TODO(minhdoan): Looking to removing this.
	w := bufio.NewWriter(conn)
FOR_LOOP:
	for {
		syncMsgType := proto_node.BlockchainSyncMessageType(payload[0])
		switch syncMsgType {
		case proto_node.GET_BLOCK:
			block := node.blockchain.FindBlock(payload[1:33])
			w.Write(block.Serialize())
			w.Flush()
		case proto_node.GET_LAST_BLOCK_HASHES:
			blockchainSyncMessage := proto_node.BlockchainSyncMessage{
				BlockHeight: len(node.blockchain.Blocks),
				BlockHashes: node.blockchain.GetBlockHashes(),
			}
			w.Write(proto_node.SerializeBlockchainSyncMessage(&blockchainSyncMessage))
			w.Flush()
		case proto_node.DONE:
			break FOR_LOOP
		}
		content, err := p2p.ReadMessageContent(conn)

		if err != nil {
			node.log.Error("Failed in reading message content from syncing node", err)
			return
		}

		msgCategory, _ := proto.GetMessageCategory(content)
		if err != nil || msgCategory != proto.NODE {
			node.log.Error("Failed in reading message category from syncing node", err)
			return
		}

		msgType, err := proto.GetMessageType(content)
		actionType := proto_node.NodeMessageType(msgType)
		if err != nil || actionType != proto_node.BLOCKCHAIN_SYNC {
			node.log.Error("Failed in reading message type from syncing node", err)
			return
		}

		payload, err = proto.GetMessagePayload(content)
		if err != nil {
			node.log.Error("Failed in reading payload from syncing node", err)
			return
		}
	}
	node.log.Info("HOORAY: Done sending info to syncing node.")
}

func (node *Node) transactionMessageHandler(msgPayload []byte) {
	txMessageType := proto_node.TransactionMessageType(msgPayload[0])

	switch txMessageType {
	case proto_node.SEND:
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the SEND messge type
		txList := new([]*blockchain.Transaction)
		err := txDecoder.Decode(txList)
		if err != nil {
			node.log.Error("Failed to deserialize transaction list", "error", err)
		}
		node.addPendingTransactions(*txList)
	case proto_node.REQUEST:
		reader := bytes.NewBuffer(msgPayload[1:])
		var txIds map[[32]byte]bool
		buf := make([]byte, 32) // 32 byte hash Id
		for {
			_, err := reader.Read(buf)
			if err != nil {
				break
			}

			var txId [32]byte
			copy(txId[:], buf)
			txIds[txId] = true
		}

		var txToReturn []*blockchain.Transaction
		for _, tx := range node.pendingTransactions {
			if txIds[tx.ID] {
				txToReturn = append(txToReturn, tx)
			}
		}
		// TODO: return the transaction list to requester
	case proto_node.UNLOCK:
		txAndProofDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the UNLOCK messge type

		txAndProofs := new([]*blockchain.Transaction)
		err := txAndProofDecoder.Decode(&txAndProofs)
		if err != nil {
			node.log.Error("Failed deserializing transaction and proofs list", "node", node)
		}
		node.log.Debug("RECEIVED UNLOCK MESSAGE", "num", len(*txAndProofs))

		node.addPendingTransactions(*txAndProofs)
	}
}

// WaitForConsensusReady ...
func (node *Node) WaitForConsensusReady(readySignal chan struct{}) {
	node.log.Debug("Waiting for Consensus ready", "node", node)

	var newBlock *blockchain.Block
	timeoutCount := 0
	for { // keep waiting for Consensus ready
		retry := false
		// TODO(minhdoan, rj): Refactor by sending signal in channel instead of waiting for 10 seconds.
		select {
		case <-readySignal:
			time.Sleep(100 * time.Millisecond) // Delay a bit so validator is catched up.
		case <-time.After(100 * time.Second):
			retry = true
			node.Consensus.ResetState()
			timeoutCount++
			node.log.Debug("Consensus timeout, retry!", "count", timeoutCount, "node", node)
		}

		//node.log.Debug("Adding new block", "currentChainSize", len(node.blockchain.Blocks), "numTxs", len(node.blockchain.GetLatestBlock().Transactions), "PrevHash", node.blockchain.GetLatestBlock().PrevBlockHash, "Hash", node.blockchain.GetLatestBlock().Hash)
		if !retry {
			if len(node.blockchain.Blocks) > NumBlocksBeforeStateBlock {
				// Generate state block and run consensus on it
				newBlock = node.blockchain.CreateStateBlock(node.UtxoPool)
			} else {
				// Normal tx block consensus
				for {
					// Once we have pending transactions we will try creating a new block
					if len(node.pendingTransactions) >= 1 {
						node.log.Debug("Start selecting transactions")
						selectedTxs, crossShardTxAndProofs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)

						if len(selectedTxs) == 0 {
							node.log.Debug("No valid transactions exist", "pendingTx", len(node.pendingTransactions))
						} else {
							node.log.Debug("Creating new block", "numAllTxs", len(selectedTxs), "numCrossTxs", len(crossShardTxAndProofs), "pendingTxs", len(node.pendingTransactions), "currentChainSize", len(node.blockchain.Blocks))

							node.transactionInConsensus = selectedTxs
							node.CrossTxsInConsensus = crossShardTxAndProofs
							newBlock = blockchain.NewBlock(selectedTxs, node.blockchain.GetLatestBlock().Hash, node.Consensus.ShardID)
							break
						}
					}
					// If not enough transactions to run Consensus,
					// periodically check whether we have enough transactions to package into block.
					time.Sleep(1 * time.Second)
				}
			}
		}

		// Send the new block to Consensus so it can be confirmed.
		if newBlock != nil {
			node.BlockChannel <- *newBlock
		}
	}
}

// This is called by consensus participants to verify the block they are running consensus on
func (node *Node) SendBackProofOfAcceptOrReject() {
	if node.ClientPeer != nil && len(node.CrossTxsToReturn) != 0 {
		node.crossTxToReturnMutex.Lock()
		proofs := []blockchain.CrossShardTxProof{}
		for _, txAndProof := range node.CrossTxsToReturn {
			proofs = append(proofs, *txAndProof.Proof)
		}
		node.CrossTxsToReturn = nil
		node.crossTxToReturnMutex.Unlock()

		node.log.Debug("SENDING PROOF TO CLIENT", "proofs", len(proofs))
		p2p.SendMessage(*node.ClientPeer, client.ConstructProofOfAcceptOrRejectMessage(proofs))
	}
}

// This is called by consensus leader to sync new blocks with other clients/nodes.
// NOTE: For now, just send to the client (basically not broadcasting)
func (node *Node) BroadcastNewBlock(newBlock *blockchain.Block) {
	if node.ClientPeer != nil {
		node.log.Debug("SENDING NEW BLOCK TO CLIENT")
		p2p.SendMessage(*node.ClientPeer, proto_node.ConstructBlocksSyncMessage([]blockchain.Block{*newBlock}))
	}
}

// This is called by consensus participants to verify the block they are running consensus on
func (node *Node) VerifyNewBlock(newBlock *blockchain.Block) bool {
	if newBlock.IsStateBlock() {
		return node.UtxoPool.VerifyStateBlock(newBlock)
	} else {
		return node.UtxoPool.VerifyTransactions(newBlock.Transactions)
	}
}

// This is called by consensus participants, after consensus is done, to:
// 1. add the new block to blockchain
// 2. [leader] move cross shard tx and proof to the list where they wait to be sent to the client
func (node *Node) PostConsensusProcessing(newBlock *blockchain.Block) {
	if newBlock.IsStateBlock() {
		// Clear out old tx blocks and put state block as genesis
		if node.db != nil {
			node.log.Info("Deleting old blocks.")
			for i := 1; i <= len(node.blockchain.Blocks); i++ {
				blockchain.Delete(node.db, strconv.Itoa(i))
			}
		}
		node.blockchain.Blocks = []*blockchain.Block{}
	}

	if node.Consensus.IsLeader {
		// Move crossTx-in-consensus into the list to be returned to client
		for _, crossTxAndProof := range node.CrossTxsInConsensus {
			crossTxAndProof.Proof.BlockHash = newBlock.Hash
			// TODO: fill in the signature proofs
		}
		if len(node.CrossTxsInConsensus) != 0 {
			node.addCrossTxsToReturn(node.CrossTxsInConsensus)
			node.CrossTxsInConsensus = []*blockchain.CrossShardTxAndProof{}
		}

		node.SendBackProofOfAcceptOrReject()
		node.BroadcastNewBlock(newBlock)
	}

	node.AddNewBlock(newBlock)
	node.UpdateUtxoAndState(newBlock)
}

func (node *Node) AddNewBlock(newBlock *blockchain.Block) {
	// Add it to blockchain
	node.blockchain.Blocks = append(node.blockchain.Blocks, newBlock)
	// Store it into leveldb.
	if node.db != nil {
		node.log.Info("Writing new block into disk.")
		newBlock.Write(node.db, strconv.Itoa(len(node.blockchain.Blocks)))
	}
}

func (node *Node) UpdateUtxoAndState(newBlock *blockchain.Block) {
	// Update UTXO pool
	if newBlock.IsStateBlock() {
		newUtxoPool := blockchain.CreateUTXOPoolFromGenesisBlock(newBlock)
		node.UtxoPool.UtxoMap = newUtxoPool.UtxoMap
	} else {
		node.UtxoPool.Update(newBlock.Transactions)
	}
	// Clear transaction-in-Consensus list
	node.transactionInConsensus = []*blockchain.Transaction{}
	if node.Consensus.IsLeader {
		node.log.Info("TX in New BLOCK", "num", len(newBlock.Transactions), "ShardId", node.UtxoPool.ShardID, "IsStateBlock", newBlock.IsStateBlock())
		node.log.Info("LEADER CURRENT UTXO", "num", node.UtxoPool.CountNumOfUtxos(), "ShardId", node.UtxoPool.ShardID)
		node.log.Info("LEADER LOCKED UTXO", "num", node.UtxoPool.CountNumOfLockedUtxos(), "ShardId", node.UtxoPool.ShardID)
	}
}
