package node

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"math/big"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/harmony-one/harmony/blockchain"
	hmy_crypto "github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/proto"
	"github.com/harmony-one/harmony/proto/client"
	"github.com/harmony-one/harmony/proto/consensus"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
	proto_node "github.com/harmony-one/harmony/proto/node"
)

const (
	// MinNumberOfTransactionsPerBlock is the min number of transaction per a block.
	MinNumberOfTransactionsPerBlock = 6000
	// MaxNumberOfTransactionsPerBlock is the max number of transaction per a block.
	MaxNumberOfTransactionsPerBlock = 8000
	// NumBlocksBeforeStateBlock is the number of blocks allowed before generating state block
	NumBlocksBeforeStateBlock = 1000
)

// MaybeBroadcastAsValidator returns if the node is a validator node.
func (node *Node) MaybeBroadcastAsValidator(content []byte) {
	if node.SelfPeer.ValidatorID > 0 && node.SelfPeer.ValidatorID <= p2p.MaxBroadCast {
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
		actionType := proto_identity.IdentityMessageType(msgType)
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
		actionType := consensus.ConsensusMessageType(msgType)
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
		actionType := proto_node.NodeMessageType(msgType)
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
		case proto_node.BlockchainSync:
			node.log.Info("NET: received message: Node/BlockchainSync")
			node.handleBlockchainSync(msgPayload, conn)
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

				os.Exit(0)
			}
		case proto_node.PING:
			node.log.Info("NET: received message: PING")
			node.pingMessageHandler(msgPayload)
		case proto_node.PONG:
			node.log.Info("NET: received message: PONG")
			node.pongMessageHandler(msgPayload)
		}
	case proto.Client:
		actionType := client.ClientMessageType(msgType)
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

// Refactor by moving this code into a sync package.
func (node *Node) handleBlockchainSync(payload []byte, conn net.Conn) {
	// TODO(minhdoan): Looking to removing this.
	w := bufio.NewWriter(conn)
FOR_LOOP:
	for {
		syncMsgType := proto_node.BlockchainSyncMessageType(payload[0])
		switch syncMsgType {
		case proto_node.GetBlock:
			block := node.blockchain.FindBlock(payload[1:33])
			w.Write(block.Serialize())
			w.Flush()
		case proto_node.GetLastBlockHashes:
			blockchainSyncMessage := proto_node.BlockchainSyncMessage{
				BlockHeight: len(node.blockchain.Blocks),
				BlockHashes: node.blockchain.GetBlockHashes(),
			}
			w.Write(proto_node.SerializeBlockchainSyncMessage(&blockchainSyncMessage))
			w.Flush()
		case proto_node.Done:
			break FOR_LOOP
		}
		content, err := p2p.ReadMessageContent(conn)

		if err != nil {
			node.log.Error("Failed in reading message content from syncing node", err)
			return
		}

		msgCategory, _ := proto.GetMessageCategory(content)
		if err != nil || msgCategory != proto.Node {
			node.log.Error("Failed in reading message category from syncing node", err)
			return
		}

		msgType, err := proto.GetMessageType(content)
		actionType := proto_node.NodeMessageType(msgType)
		if err != nil || actionType != proto_node.BlockchainSync {
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
	case proto_node.Send:
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the Send messge type
		txList := new([]*blockchain.Transaction)
		err := txDecoder.Decode(txList)
		if err != nil {
			node.log.Error("Failed to deserialize transaction list", "error", err)
		}
		node.addPendingTransactions(*txList)
	case proto_node.Request:
		reader := bytes.NewBuffer(msgPayload[1:])
		txIDs := make(map[[32]byte]bool)
		buf := make([]byte, 32) // 32 byte hash Id
		for {
			_, err := reader.Read(buf)
			if err != nil {
				break
			}

			var txID [32]byte
			copy(txID[:], buf)
			txIDs[txID] = true
		}

		var txToReturn []*blockchain.Transaction
		for _, tx := range node.pendingTransactions {
			if txIDs[tx.ID] {
				txToReturn = append(txToReturn, tx)
			}
		}
		// TODO: return the transaction list to requester
	case proto_node.Unlock:
		txAndProofDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the Unlock messge type

		txAndProofs := new([]*blockchain.Transaction)
		err := txAndProofDecoder.Decode(&txAndProofs)
		if err != nil {
			node.log.Error("Failed deserializing transaction and proofs list", "node", node)
		}
		node.log.Debug("RECEIVED Unlock MESSAGE", "num", len(*txAndProofs))

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
					if len(node.pendingTransactions) >= MaxNumberOfTransactionsPerBlock {
						node.log.Debug("Start selecting transactions")
						selectedTxs, crossShardTxAndProofs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)

						if len(selectedTxs) < MinNumberOfTransactionsPerBlock {
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

// WaitForConsensusReady ...
func (node *Node) WaitForConsensusReadyAccount(readySignal chan struct{}) {
	node.log.Debug("Waiting for Consensus ready", "node", node)

	var newBlock *types.Block
	timeoutCount := 0
	for { // keep waiting for Consensus ready
		retry := false
		select {
		case <-readySignal:
			time.Sleep(100 * time.Millisecond) // Delay a bit so validator is catched up.
		case <-time.After(100 * time.Second):
			retry = true
			node.Consensus.ResetState()
			timeoutCount++
			node.log.Debug("Consensus timeout, retry!", "count", timeoutCount, "node", node)
		}

		if !retry {
			// Normal tx block consensus
			// TODO: add new block generation logic
			txs := make([]*types.Transaction, 100)
			for i, _ := range txs {
				randomUserKey, _ := crypto.GenerateKey()
				randomUserAddress := crypto.PubkeyToAddress(randomUserKey.PublicKey)
				tx, _ := types.SignTx(types.NewTransaction(node.worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(node.testBankKey.PublicKey)), randomUserAddress, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, node.testBankKey)
				txs[i] = tx
			}
			node.worker.CommitTransactions(txs, crypto.PubkeyToAddress(node.testBankKey.PublicKey))
			newBlock = node.worker.Commit()

			// If not enough transactions to run Consensus,
			// periodically check whether we have enough transactions to package into block.
			time.Sleep(1 * time.Second)
		}

		// Send the new block to Consensus so it can be confirmed.
		if newBlock != nil {
			node.BlockChannelAccount <- newBlock
		}
	}
}

// SendBackProofOfAcceptOrReject is called by consensus participants to verify the block they are running consensus on
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

// BroadcastNewBlock is called by consensus leader to sync new blocks with other clients/nodes.
// NOTE: For now, just send to the client (basically not broadcasting)
func (node *Node) BroadcastNewBlock(newBlock *blockchain.Block) {
	if node.ClientPeer != nil {
		node.log.Debug("NET: SENDING NEW BLOCK TO CLIENT")
		p2p.SendMessage(*node.ClientPeer, proto_node.ConstructBlocksSyncMessage([]blockchain.Block{*newBlock}))
	}
}

// VerifyNewBlock is called by consensus participants to verify the block they are running consensus on
func (node *Node) VerifyNewBlock(newBlock *blockchain.Block) bool {
	if newBlock.AccountBlock != nil {
		accountBlock := new(types.Block)
		err := rlp.DecodeBytes(newBlock.AccountBlock, accountBlock)
		if err != nil {
			node.log.Error("Failed decoding the block with RLP")
		}
		return node.VerifyNewBlockAccount(accountBlock)
	}
	if newBlock.IsStateBlock() {
		return node.UtxoPool.VerifyStateBlock(newBlock)
	}
	return node.UtxoPool.VerifyTransactions(newBlock.Transactions)
}

// VerifyNewBlock is called by consensus participants to verify the block (account model) they are running consensus on
func (node *Node) VerifyNewBlockAccount(newBlock *types.Block) bool {
	fmt.Println("VerifyingNNNNNNNNNNNNNN")

	fmt.Println("BALANCE 1")
	fmt.Println(node.worker.GetCurrentState().GetBalance(crypto.PubkeyToAddress(node.testBankKey.PublicKey)))
	return node.Chain.ValidateNewBlock(newBlock, crypto.PubkeyToAddress(node.testBankKey.PublicKey))
}

// PostConsensusProcessing is called by consensus participants, after consensus is done, to:
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

// AddNewBlock is usedd to add new block into the blockchain.
func (node *Node) AddNewBlock(newBlock *blockchain.Block) {
	// Add it to blockchain
	node.blockchain.Blocks = append(node.blockchain.Blocks, newBlock)
	// Store it into leveldb.
	if node.db != nil {
		node.log.Info("Writing new block into disk.")
		newBlock.Write(node.db, strconv.Itoa(len(node.blockchain.Blocks)))
	}
}

// UpdateUtxoAndState updates Utxo and state.
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
		node.log.Info("TX in New BLOCK", "num", len(newBlock.Transactions), "ShardID", node.UtxoPool.ShardID, "IsStateBlock", newBlock.IsStateBlock())
		node.log.Info("LEADER CURRENT UTXO", "num", node.UtxoPool.CountNumOfUtxos(), "ShardID", node.UtxoPool.ShardID)
		node.log.Info("LEADER LOCKED UTXO", "num", node.UtxoPool.CountNumOfLockedUtxos(), "ShardID", node.UtxoPool.ShardID)
	}
}

func (node *Node) pingMessageHandler(msgPayload []byte) {
	ping, err := proto_node.GetPingMessage(msgPayload)
	if err != nil {
		node.log.Error("Can't get Ping Message")
		return
	}
	node.log.Info("Ping", "Msg", ping)

	peer := new(p2p.Peer)
	peer.Ip = ping.Node.IP
	peer.Port = ping.Node.Port

	peer.PubKey = hmy_crypto.Ed25519Curve.Point()
	err = peer.PubKey.UnmarshalBinary(ping.Node.PubKey[:])
	if err != nil {
		node.log.Error("UnmarshalBinary Failed", "error", err)
		return
	}

	// Add to Node's peer list
	count := node.AddPeers([]p2p.Peer{*peer})

	// Send a Pong message back
	peers := make([]p2p.Peer, 0)
	count = 0
	node.Neighbors.Range(func(k, v interface{}) bool {
		if p, ok := v.(p2p.Peer); ok {
			peers = append(peers, p)
			count++
			return true
		} else {
			return false
		}
	})
	pong := proto_node.NewPongMessage(peers)
	buffer := pong.ConstructPongMessage()

	p2p.SendMessage(*peer, buffer)

	// TODO: broadcast pong messages to all neighbors

	return
}

func (node *Node) pongMessageHandler(msgPayload []byte) {
	pong, err := proto_node.GetPongMessage(msgPayload)
	if err != nil {
		node.log.Error("Can't get Pong Message")
		return
	}
	//	node.log.Info("Pong", "Msg", pong)
	node.State = NodeJoinedShard

	peers := make([]p2p.Peer, 0)

	for _, p := range pong.Peers {
		peer := new(p2p.Peer)
		peer.Ip = p.IP
		peer.Port = p.Port

		peer.PubKey = hmy_crypto.Ed25519Curve.Point()
		err = peer.PubKey.UnmarshalBinary(p.PubKey[:])
		if err != nil {
			node.log.Error("UnmarshalBinary Failed", "error", err)
			continue
		}
		peers = append(peers, *peer)
	}

	node.AddPeers(peers)
	// TODO: add public key to consensus.pubkeys

	return
}
