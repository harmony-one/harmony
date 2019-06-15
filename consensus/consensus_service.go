package consensus

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/harmony-one/harmony/crypto/hash"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	"golang.org/x/crypto/sha3"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/profiler"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// WaitForNewRandomness listens to the RndChannel to receive new VDF randomness.
func (consensus *Consensus) WaitForNewRandomness() {
	go func() {
		for {
			vdfOutput := <-consensus.RndChannel
			consensus.pendingRnds = append(consensus.pendingRnds, vdfOutput)
		}
	}()
}

// GetNextRnd returns the oldest available randomness along with the hash of the block there randomness preimage is committed.
func (consensus *Consensus) GetNextRnd() ([32]byte, [32]byte, error) {
	if len(consensus.pendingRnds) == 0 {
		return [32]byte{}, [32]byte{}, errors.New("No available randomness")
	}
	vdfOutput := consensus.pendingRnds[0]

	//pop the first vdfOutput from the list
	consensus.pendingRnds = consensus.pendingRnds[1:]

	rnd := [32]byte{}
	blockHash := [32]byte{}
	copy(rnd[:], vdfOutput[:32])
	copy(blockHash[:], vdfOutput[32:])
	return rnd, blockHash, nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (consensus *Consensus) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	// TODO: update with new fields
	if err := rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra,
	}); err != nil {
		ctxerror.Warn(utils.GetLogger(), err, "rlp.Encode failed")
	}
	hasher.Sum(hash[:0])
	return hash
}

// Seal is to seal final block.
func (consensus *Consensus) Seal(chain consensus_engine.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	// TODO: implement final block sealing
	return nil
}

// Author returns the author of the block header.
func (consensus *Consensus) Author(header *types.Header) (common.Address, error) {
	// TODO: implement this
	return common.Address{}, nil
}

// Prepare is to prepare ...
// TODO(RJ): fix it.
func (consensus *Consensus) Prepare(chain consensus_engine.ChainReader, header *types.Header) error {
	// TODO: implement prepare method
	return nil
}

// Populates the common basic fields for all consensus message.
func (consensus *Consensus) populateMessageFields(request *msg_pb.ConsensusRequest) {
	request.ViewId = consensus.viewID
	request.BlockNum = consensus.blockNum
	request.ShardId = consensus.ShardID

	// 32 byte block hash
	request.BlockHash = consensus.blockHash[:]

	// sender address
	request.SenderPubkey = consensus.PubKey.Serialize()
	consensus.getLogger().Debug("[populateMessageFields]", "SenderKey", consensus.PubKey.SerializeToHexStr())
}

// Signs the consensus message and returns the marshaled message.
func (consensus *Consensus) signAndMarshalConsensusMessage(message *msg_pb.Message) ([]byte, error) {
	err := consensus.signConsensusMessage(message)
	if err != nil {
		return []byte{}, err
	}

	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return []byte{}, err
	}
	return marshaledMessage, nil
}

// SetLeaderPubKey deserialize the public key of consensus leader
func (consensus *Consensus) SetLeaderPubKey(k []byte) error {
	consensus.leader.ConsensusPubKey = &bls.PublicKey{}
	return consensus.leader.ConsensusPubKey.Deserialize(k)
}

// GetLeaderPubKey returns the public key of consensus leader
func (consensus *Consensus) GetLeaderPubKey() *bls.PublicKey {
	return consensus.leader.ConsensusPubKey
}

// GetNodeIDs returns Node IDs of all nodes in the same shard
func (consensus *Consensus) GetNodeIDs() []libp2p_peer.ID {
	nodes := make([]libp2p_peer.ID, 0)
	nodes = append(nodes, consensus.host.GetID())
	consensus.validators.Range(func(k, v interface{}) bool {
		if peer, ok := v.(p2p.Peer); ok {
			nodes = append(nodes, peer.PeerID)
			return true
		}
		return false
	})
	return nodes
}

// GetViewID returns the consensus ID
func (consensus *Consensus) GetViewID() uint32 {
	return consensus.viewID
}

// DebugPrintPublicKeys print all the PublicKeys in string format in Consensus
func (consensus *Consensus) DebugPrintPublicKeys() {
	for _, k := range consensus.PublicKeys {
		str := fmt.Sprintf("%s", hex.EncodeToString(k.Serialize()))
		utils.GetLogInstance().Debug("pk:", "string", str)
	}

	utils.GetLogInstance().Debug("PublicKeys:", "#", len(consensus.PublicKeys))
}

// UpdatePublicKeys updates the PublicKeys variable, protected by a mutex
func (consensus *Consensus) UpdatePublicKeys(pubKeys []*bls.PublicKey) int {
	consensus.pubKeyLock.Lock()
	consensus.PublicKeys = append(pubKeys[:0:0], pubKeys...)
	consensus.CommitteePublicKeys = map[string]bool{}
	for _, pubKey := range consensus.PublicKeys {
		consensus.CommitteePublicKeys[pubKey.SerializeToHexStr()] = true
	}
	// TODO: use pubkey to identify leader rather than p2p.Peer.
	consensus.leader = p2p.Peer{ConsensusPubKey: pubKeys[0]}
	consensus.LeaderPubKey = pubKeys[0]
	prepareBitmap, err := bls_cosi.NewMask(consensus.PublicKeys, consensus.LeaderPubKey)
	if err == nil {
		consensus.prepareBitmap = prepareBitmap
	}

	commitBitmap, err := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	if err == nil {
		consensus.commitBitmap = commitBitmap
	}

	utils.GetLogInstance().Info("My Leader", "info", consensus.LeaderPubKey.SerializeToHexStr())
	utils.GetLogInstance().Info("My Committee", "info", consensus.PublicKeys)
	consensus.pubKeyLock.Unlock()
	// reset states after update public keys
	consensus.ResetState()
	consensus.ResetViewChangeState()

	return len(consensus.PublicKeys)
}

// NewFaker returns a faker consensus.
func NewFaker() *Consensus {
	return &Consensus{}
}

// VerifyHeader checks whether a header conforms to the consensus rules of the bft engine.
func (consensus *Consensus) VerifyHeader(chain consensus_engine.ChainReader, header *types.Header, seal bool) error {
	parentHeader := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parentHeader == nil {
		return consensus_engine.ErrUnknownAncestor
	}
	if seal {
		if err := consensus.VerifySeal(chain, header); err != nil {
			return err
		}
	}
	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (consensus *Consensus) VerifyHeaders(chain consensus_engine.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort, results := make(chan struct{}), make(chan error, len(headers))
	for i := 0; i < len(headers); i++ {
		results <- nil
	}
	return abort, results
}

// VerifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (consensus *Consensus) VerifySeal(chain consensus_engine.ChainReader, header *types.Header) error {
	return nil
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (consensus *Consensus) Finalize(chain consensus_engine.ChainReader, header *types.Header, state *state.DB, txs []*types.Transaction, receipts []*types.Receipt) (*types.Block, error) {
	// Accumulate any block and uncle rewards and commit the final state root
	// Header seems complete, assemble into a block and return
	if err := accumulateRewards(chain, state, header); err != nil {
		return nil, ctxerror.New("cannot pay block reward").WithCause(err)
	}
	header.Root = state.IntermediateRoot(false)
	return types.NewBlock(header, txs, receipts), nil
}

// Sign on the hash of the message
func (consensus *Consensus) signMessage(message []byte) []byte {
	hash := hash.Keccak256(message)
	signature := consensus.priKey.SignHash(hash[:])
	return signature.Serialize()
}

// Sign on the consensus message signature field.
func (consensus *Consensus) signConsensusMessage(message *msg_pb.Message) error {
	message.Signature = nil
	// TODO: use custom serialization method rather than protobuf
	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature
	return nil
}

// GetValidatorPeers returns list of validator peers.
func (consensus *Consensus) GetValidatorPeers() []p2p.Peer {
	validatorPeers := make([]p2p.Peer, 0)

	consensus.validators.Range(func(k, v interface{}) bool {
		if peer, ok := v.(p2p.Peer); ok {
			validatorPeers = append(validatorPeers, peer)
			return true
		}
		return false
	})

	return validatorPeers
}

// GetPrepareSigsArray returns the signatures for prepare as a array
func (consensus *Consensus) GetPrepareSigsArray() []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.prepareSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetCommitSigsArray returns the signatures for commit as a array
func (consensus *Consensus) GetCommitSigsArray() []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.commitSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetBhpSigsArray returns the signatures for prepared message in viewchange
func (consensus *Consensus) GetBhpSigsArray() []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.bhpSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetNilSigsArray returns the signatures for nil prepared message in viewchange
func (consensus *Consensus) GetNilSigsArray() []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.nilSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetViewIDSigsArray returns the signatures for viewID in viewchange
func (consensus *Consensus) GetViewIDSigsArray() []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.viewIDSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// ResetState resets the state of the consensus
func (consensus *Consensus) ResetState() {
	consensus.phase = Announce
	consensus.blockHash = [32]byte{}
	consensus.prepareSigs = map[string]*bls.Sign{}
	consensus.commitSigs = map[string]*bls.Sign{}

	prepareBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.LeaderPubKey)
	commitBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.LeaderPubKey)
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap
	consensus.aggregatedPrepareSig = nil
	consensus.aggregatedCommitSig = nil
}

// Returns a string representation of this consensus
func (consensus *Consensus) String() string {
	var duty string
	if nodeconfig.GetDefaultConfig().IsLeader() {
		duty = "LDR" // leader
	} else {
		duty = "VLD" // validator
	}
	return fmt.Sprintf("[duty:%s, PubKey:%s, ShardID:%v]",
		duty, consensus.PubKey.SerializeToHexStr(), consensus.ShardID)
}

// ToggleConsensusCheck flip the flag of whether ignore viewID check during consensus process
func (consensus *Consensus) ToggleConsensusCheck() {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.ignoreViewIDCheck = !consensus.ignoreViewIDCheck
}

// IsValidatorInCommittee returns whether the given validator BLS address is part of my committee
func (consensus *Consensus) IsValidatorInCommittee(pubKey *bls.PublicKey) bool {
	_, ok := consensus.CommitteePublicKeys[pubKey.SerializeToHexStr()]
	return ok
}

// Verify the signature of the message are valid from the signer's public key.
func verifyMessageSig(signerPubKey *bls.PublicKey, message *msg_pb.Message) error {
	signature := message.Signature
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}

	msgSig := bls.Sign{}
	err = msgSig.Deserialize(signature)
	if err != nil {
		return err
	}
	msgHash := hash.Keccak256(messageBytes)
	if !msgSig.VerifyHash(signerPubKey, msgHash[:]) {
		return errors.New("failed to verify the signature")
	}
	message.Signature = signature
	return nil
}

// verifySenderKey verifys the message senderKey is properly signed and senderAddr is valid
func (consensus *Consensus) verifySenderKey(msg *msg_pb.Message) (*bls.PublicKey, error) {
	consensusMsg := msg.GetConsensus()
	senderKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}

	if !consensus.IsValidatorInCommittee(senderKey) {
		return nil, fmt.Errorf("Validator %s is not in committee", senderKey.SerializeToHexStr())
	}
	return senderKey, nil
}

func (consensus *Consensus) verifyViewChangeSenderKey(msg *msg_pb.Message) (*bls.PublicKey, error) {
	vcMsg := msg.GetViewchange()
	senderKey, err := bls_cosi.BytesToBlsPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}

	if !consensus.IsValidatorInCommittee(senderKey) {
		return nil, fmt.Errorf("Validator %s is not in committee", senderKey.SerializeToHexStr())
	}
	return senderKey, nil
}

// SetViewID set the viewID to the height of the blockchain
func (consensus *Consensus) SetViewID(height uint32) {
	consensus.viewID = height
}

// RegisterPRndChannel registers the channel for receiving randomness preimage from DRG protocol
func (consensus *Consensus) RegisterPRndChannel(pRndChannel chan []byte) {
	consensus.PRndChannel = pRndChannel
}

// RegisterRndChannel registers the channel for receiving final randomness from DRG protocol
func (consensus *Consensus) RegisterRndChannel(rndChannel chan [64]byte) {
	consensus.RndChannel = rndChannel
}

// Check viewID, caller's responsibility to hold lock when change ignoreViewIDCheck
func (consensus *Consensus) checkViewID(msg *PbftMessage) error {
	// just ignore consensus check for the first time when node join
	if consensus.ignoreViewIDCheck {
		//in syncing mode, node accepts incoming messages without viewID/leaderKey checking
		//so only set mode to normal when new node enters consensus and need checking viewID
		consensus.mode.SetMode(Normal)
		consensus.viewID = msg.ViewID
		consensus.mode.SetViewID(msg.ViewID)
		consensus.LeaderPubKey = msg.SenderPubkey
		consensus.ignoreViewIDCheck = false
		consensus.consensusTimeout[timeoutConsensus].Start()
		utils.GetLogger().Debug("viewID and leaderKey override", "viewID", consensus.viewID, "leaderKey", consensus.LeaderPubKey.SerializeToHexStr()[:20])
		utils.GetLogger().Debug("Start consensus timer", "viewID", consensus.viewID, "block", consensus.blockNum)
		return nil
	} else if msg.ViewID > consensus.viewID {
		return consensus_engine.ErrViewIDNotMatch
	} else if msg.ViewID < consensus.viewID {
		return errors.New("view ID belongs to the past")
	}
	return nil
}

// SetBlockNum sets the blockNum in consensus object, called at node bootstrap
func (consensus *Consensus) SetBlockNum(blockNum uint64) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.blockNum = blockNum
}

// read the payload for signature and bitmap; offset is the beginning position of reading
func (consensus *Consensus) readSignatureBitmapPayload(recvPayload []byte, offset int) (*bls.Sign, *bls_cosi.Mask, error) {
	if offset+96 > len(recvPayload) {
		return nil, nil, errors.New("payload not have enough length")
	}
	payload := append(recvPayload[:0:0], recvPayload...)
	//#### Read payload data
	// 96 byte of multi-sig
	multiSig := payload[offset : offset+96]
	offset += 96
	// bitmap
	bitmap := payload[offset:]
	//#### END Read payload data

	aggSig := bls.Sign{}
	err := aggSig.Deserialize(multiSig)
	if err != nil {
		return nil, nil, errors.New("unable to deserialize multi-signature from payload")
	}
	mask, err := bls_cosi.NewMask(consensus.PublicKeys, nil)
	if err != nil {
		utils.GetLogInstance().Warn("onNewView unable to setup mask for prepared message", "err", err)
		return nil, nil, errors.New("unable to setup mask from payload")
	}
	if err := mask.SetMask(bitmap); err != nil {
		ctxerror.Warn(utils.GetLogger(), err, "mask.SetMask failed")
	}
	return &aggSig, mask, nil
}

func (consensus *Consensus) reportMetrics(block types.Block) {
	endTime := time.Now()
	timeElapsed := endTime.Sub(startTime)
	numOfTxs := len(block.Transactions())
	tps := float64(numOfTxs) / timeElapsed.Seconds()
	utils.GetLogInstance().Info("TPS Report",
		"numOfTXs", numOfTxs,
		"startTime", startTime,
		"endTime", endTime,
		"timeElapsed", timeElapsed,
		"TPS", tps,
		"consensus", consensus)

	// Post metrics
	profiler := profiler.GetProfiler()
	if profiler.MetricsReportURL == "" {
		return
	}

	txHashes := []string{}
	for i, end := 0, len(block.Transactions()); i < 3 && i < end; i++ {
		txHash := block.Transactions()[end-1-i].Hash()
		txHashes = append(txHashes, hex.EncodeToString(txHash[:]))
	}
	metrics := map[string]interface{}{
		"key":             hex.EncodeToString(consensus.PubKey.Serialize()),
		"tps":             tps,
		"txCount":         numOfTxs,
		"nodeCount":       len(consensus.PublicKeys) + 1,
		"latestBlockHash": hex.EncodeToString(consensus.blockHash[:]),
		"latestTxHashes":  txHashes,
		"blockLatency":    int(timeElapsed / time.Millisecond),
	}
	profiler.LogMetrics(metrics)
}

// logger returns a sub-logger with consensus contexts added.
func (consensus *Consensus) logger(logger log.Logger) log.Logger {
	return logger.New(
		"myBlock", consensus.blockNum,
		"myViewID", consensus.viewID,
		"phase", consensus.phase,
		"mode", consensus.mode.Mode(),
	)
}

// getLogger returns logger for consensus contexts added
func (consensus *Consensus) getLogger() log.Logger {
	logger := consensus.logger(utils.GetLogInstance())
	return utils.WithCallerSkip(logger, 1)
}
