package consensus

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const (
	// ValidPayloadLength is the valid length for viewchange payload
	ValidPayloadLength = 32 + bls.BLSSignatureSizeInBytes
)

// viewChange encapsulate all the view change related data structure and functions
type viewChange struct {
	// Commits collected from view change
	// for each viewID, we need keep track of corresponding sigs and bitmap
	// until one of the viewID has enough votes (>=2f+1)
	// after one of viewID has enough votes, we can reset and clean the map
	// honest nodes will never double votes on different viewID
	// bhpSigs: blockHashPreparedSigs is the signature on m1 type message
	bhpSigs map[uint64]map[string]*bls_core.Sign
	// nilSigs: there is no prepared message when view change,
	// it's signature on m2 type (i.e. nil) messages
	nilSigs      map[uint64]map[string]*bls_core.Sign
	viewIDSigs   map[uint64]map[string]*bls_core.Sign
	bhpBitmap    map[uint64]*bls_cosi.Mask
	nilBitmap    map[uint64]*bls_cosi.Mask
	viewIDBitmap map[uint64]*bls_cosi.Mask
	newViewMsg   map[uint64]map[string]uint64

	m1Payload []byte // message payload for type m1 := |vcBlockHash|prepared_agg_sigs|prepared_bitmap|, new leader only need one

	verifyBlock        VerifyBlockFunc
	viewChangeDuration time.Duration
}

// newViewChange returns a new viewChange object
func newViewChange() *viewChange {
	vc := viewChange{}
	vc.Reset()
	return &vc
}

// AddViewIDKeyIfNotExist ..
func (vc *viewChange) AddViewIDKeyIfNotExist(viewID uint64, members multibls.PublicKeys) {
	if _, ok := vc.bhpSigs[viewID]; !ok {
		vc.bhpSigs[viewID] = map[string]*bls_core.Sign{}
	}
	if _, ok := vc.nilSigs[viewID]; !ok {
		vc.nilSigs[viewID] = map[string]*bls_core.Sign{}
	}
	if _, ok := vc.viewIDSigs[viewID]; !ok {
		vc.viewIDSigs[viewID] = map[string]*bls_core.Sign{}
	}
	if _, ok := vc.bhpBitmap[viewID]; !ok {
		bhpBitmap := bls_cosi.NewMask(members)
		vc.bhpBitmap[viewID] = bhpBitmap
	}
	if _, ok := vc.nilBitmap[viewID]; !ok {
		nilBitmap := bls_cosi.NewMask(members)
		vc.nilBitmap[viewID] = nilBitmap
	}
	if _, ok := vc.viewIDBitmap[viewID]; !ok {
		viewIDBitmap := bls_cosi.NewMask(members)
		vc.viewIDBitmap[viewID] = viewIDBitmap
	}
}

// Reset resets the state for viewChange.
func (vc *viewChange) Reset() {
	vc.m1Payload = []byte{}
	vc.bhpSigs = map[uint64]map[string]*bls_core.Sign{}
	vc.nilSigs = map[uint64]map[string]*bls_core.Sign{}
	vc.viewIDSigs = map[uint64]map[string]*bls_core.Sign{}
	vc.bhpBitmap = map[uint64]*bls_cosi.Mask{}
	vc.nilBitmap = map[uint64]*bls_cosi.Mask{}
	vc.viewIDBitmap = map[uint64]*bls_cosi.Mask{}
	vc.newViewMsg = map[uint64]map[string]uint64{}
}

// GetPreparedBlock returns the prepared block or nil if not found
func (vc *viewChange) GetPreparedBlock(fbftlog *FBFTLog) ([]byte, []byte) {
	if vc.isM1PayloadEmpty() {
		return nil, nil
	}

	blockHash := [32]byte{}
	// First 32 bytes of m1 payload is the correct block hash
	copy(blockHash[:], vc.GetM1Payload())

	if !fbftlog.IsBlockVerified(blockHash) {
		return nil, nil
	}

	if block := fbftlog.GetBlockByHash(blockHash); block != nil {
		encodedBlock, err := rlp.EncodeToBytes(block)
		if err != nil || len(encodedBlock) == 0 {
			return nil, nil
		}
		return vc.m1Payload, encodedBlock
	}

	return nil, nil
}

// GetM2Bitmap returns the nilBitmap as M2Bitmap
func (vc *viewChange) GetM2Bitmap(viewID uint64) ([]byte, []byte) {
	sig2arr := []*bls_core.Sign{}
	for _, sig := range vc.nilSigs[viewID] {
		sig2arr = append(sig2arr, sig)
	}

	if len(sig2arr) > 0 {
		m2Sig := bls_cosi.AggregateSig(sig2arr)
		vc.getLogger().Info().Int("len", len(sig2arr)).Msg("[GetM2Bitmap] M2 (NIL) type signatures")
		return m2Sig.Serialize(), vc.nilBitmap[viewID].Bitmap
	}
	vc.getLogger().Info().Uint64("viewID", viewID).Msg("[GetM2Bitmap] No M2 (NIL) type signatures")
	return nil, nil
}

// GetM3Bitmap returns the viewIDBitmap as M3Bitmap
func (vc *viewChange) GetM3Bitmap(viewID uint64) ([]byte, []byte) {
	sig3arr := []*bls_core.Sign{}
	for _, sig := range vc.viewIDSigs[viewID] {
		sig3arr = append(sig3arr, sig)
	}
	// even we check here for safty, m3 type signatures must >= 2f+1
	if len(sig3arr) > 0 {
		m3Sig := bls_cosi.AggregateSig(sig3arr)
		vc.getLogger().Info().Int("len", len(sig3arr)).Msg("[GetM3Bitmap] M3 (ViewID) type signatures")
		return m3Sig.Serialize(), vc.viewIDBitmap[viewID].Bitmap
	}
	vc.getLogger().Info().Uint64("viewID", viewID).Msg("[GetM3Bitmap] No M3 (ViewID) type signatures")
	return nil, nil
}

// VerifyNewViewMsg returns true if the new view message is valid
func (vc *viewChange) VerifyNewViewMsg(recvMsg *FBFTMessage) (*types.Block, error) {
	if recvMsg.M3AggSig == nil || recvMsg.M3Bitmap == nil {
		return nil, errors.New("[VerifyNewViewMsg] M3AggSig or M3Bitmap is nil")
	}

	senderKey := recvMsg.SenderPubkeys[0]
	senderKeyStr := senderKey.Bytes.Hex()

	// first time received the new view message
	_, ok := vc.newViewMsg[recvMsg.ViewID]
	if !ok {
		newViewMap := map[string]uint64{}
		vc.newViewMsg[recvMsg.ViewID] = newViewMap
	}

	_, ok = vc.newViewMsg[recvMsg.ViewID][senderKeyStr]
	if ok {
		vc.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MyBlockNum", vc.newViewMsg[recvMsg.ViewID][senderKeyStr]).
			Msg("[VerifyNewViewMsg] redundant NewView Message")
	}

	vc.newViewMsg[recvMsg.ViewID][senderKeyStr] = recvMsg.BlockNum

	m3Sig := recvMsg.M3AggSig
	m3Mask := recvMsg.M3Bitmap

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)

	if !m3Sig.VerifyHash(m3Mask.AggregatePublic, viewIDBytes) {
		vc.getLogger().Warn().
			Bytes("viewIDBytes", viewIDBytes).
			Interface("AggregatePublic", m3Mask.AggregatePublic).
			Msg("m3Sig.VerifyHash Failed")
		return nil, errors.New("[VerifyNewViewMsg] Unable to Verify Aggregated Signature of M3 (ViewID) payload")
	}

	m2Mask := recvMsg.M2Bitmap
	if recvMsg.M2AggSig != nil {
		m2Sig := recvMsg.M2AggSig
		if !m2Sig.VerifyHash(m2Mask.AggregatePublic, NIL) {
			vc.getLogger().Warn().
				Bytes("NIL", NIL).
				Interface("AggregatePublic", m2Mask.AggregatePublic).
				Msg("m2Sig.VerifyHash Failed")
			return nil, errors.New("[VerifyNewViewMsg] Unable to Verify Aggregated Signature of M2 (NIL) payload")
		}
	}

	// check when M3 sigs > M2 sigs, then M1 (recvMsg.Payload) should not be empty
	preparedBlock := &types.Block{}
	if len(recvMsg.Payload) >= ValidPayloadLength && len(recvMsg.Block) != 0 {
		if err := rlp.DecodeBytes(recvMsg.Block, preparedBlock); err != nil {
			return nil, errors.New("[VerifyNewViewMsg] Unparseable prepared block data")
		}

		blockHash := recvMsg.Payload[:32]
		preparedBlockHash := preparedBlock.Hash()
		if !bytes.Equal(preparedBlockHash[:], blockHash) {
			return nil, errors.New("[VerifyNewViewMsg] Prepared block hash doesn't match msg block hash")
		}
		if err := vc.verifyBlock(preparedBlock); err != nil {
			return nil, err
		}
		return preparedBlock, nil
	}
	return nil, nil
}

var (
	errDupM1           = errors.New("received M1 (prepared) message already")
	errDupM2           = errors.New("received M2 (NIL) message already")
	errDupM3           = errors.New("received M3 (ViewID) message already")
	errVerifyM1        = errors.New("failed to verfiy signature for M1 message")
	errVerifyM2        = errors.New("failed to verfiy signature for M2 message")
	errM1Payload       = errors.New("failed to verify multi signature for M1 prepared payload")
	errNoQuorum        = errors.New("no quorum on M1 (prepared) payload")
	errIncorrectSender = errors.New("multiple senders not allowed")
)

// ProcessViewChangeMsg process the view change message after verification
func (vc *viewChange) ProcessViewChangeMsg(
	fbftlog *FBFTLog,
	decider quorum.Decider,
	recvMsg *FBFTMessage,
) error {
	preparedBlock := &types.Block{}
	if !recvMsg.HasSingleSender() {
		return errIncorrectSender
	}
	senderKey := recvMsg.SenderPubkeys[0]
	senderKeyStr := senderKey.Bytes.Hex()

	// check and add viewID (m3 type) message signature
	if _, ok := vc.viewIDSigs[recvMsg.ViewID][senderKeyStr]; ok {
		return errDupM3
	}

	if len(recvMsg.Payload) >= ValidPayloadLength && len(recvMsg.Block) != 0 {
		if err := rlp.DecodeBytes(recvMsg.Block, preparedBlock); err != nil {
			return err
		}
		if err := vc.verifyBlock(preparedBlock); err != nil {
			return err
		}
		_, ok := vc.bhpSigs[recvMsg.ViewID][senderKeyStr]
		if ok {
			return errDupM1
		}
		if !recvMsg.ViewchangeSig.VerifyHash(senderKey.Object, recvMsg.Payload) {
			return errVerifyM1
		}
		blockHash := recvMsg.Payload[:32]

		aggSig, mask, err := chain.ReadSignatureBitmapByPublicKeys(recvMsg.Payload[32:], decider.Participants())
		if err != nil {
			return err
		}

		if !decider.IsQuorumAchievedByMask(mask) {
			return errNoQuorum
		}

		if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
			return errM1Payload
		}

		vc.getLogger().Info().Uint64("viewID", recvMsg.ViewID).
			Str("validatorPubKey", senderKeyStr).
			Msg("[ProcessViewChangeMsg] Add M1 (prepared) type message")

		if _, ok := vc.bhpSigs[recvMsg.ViewID]; !ok {
			vc.bhpSigs[recvMsg.ViewID] = map[string]*bls_core.Sign{}
		}
		vc.bhpSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewchangeSig
		if _, ok := vc.bhpBitmap[recvMsg.ViewID]; !ok {
			bhpBitmap := bls_cosi.NewMask(decider.Participants())
			vc.bhpBitmap[recvMsg.ViewID] = bhpBitmap
		}
		vc.bhpBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true) // Set the bitmap indicating that this validator signed.

		vc.getLogger().Info().Uint64("viewID", recvMsg.ViewID).
			Str("validatorPubKey", senderKeyStr).
			Msg("[ProcessViewChangeMsg] Add M3 (ViewID) type message")

		if _, ok := vc.viewIDSigs[recvMsg.ViewID]; !ok {
			vc.viewIDSigs[recvMsg.ViewID] = map[string]*bls_core.Sign{}
		}
		vc.viewIDSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewidSig

		if _, ok := vc.viewIDBitmap[recvMsg.ViewID]; !ok {
			viewIDBitmap := bls_cosi.NewMask(decider.Participants())
			vc.viewIDBitmap[recvMsg.ViewID] = viewIDBitmap
		}
		// Set the bitmap indicating that this validator signed.
		vc.viewIDBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true)

		if vc.isM1PayloadEmpty() {
			vc.m1Payload = append(recvMsg.Payload[:0:0], recvMsg.Payload...)
			// create prepared message for new leader
			preparedMsg := FBFTMessage{
				MessageType: msg_pb.MessageType_PREPARED,
				ViewID:      recvMsg.ViewID,
				BlockNum:    recvMsg.BlockNum,
			}
			preparedMsg.BlockHash = common.Hash{}
			copy(preparedMsg.BlockHash[:], recvMsg.Payload[:32])
			preparedMsg.Payload = make([]byte, len(recvMsg.Payload)-32)
			copy(preparedMsg.Payload[:], recvMsg.Payload[32:])

			preparedMsg.SenderPubkeys = []*bls.PublicKeyWrapper{recvMsg.LeaderPubkey}
			vc.getLogger().Info().Msg("[ProcessViewChangeMsg] New Leader Prepared Message Added")
			fbftlog.AddVerifiedMessage(&preparedMsg)
			fbftlog.AddBlock(preparedBlock)
		}
		return nil
	}

	_, ok := vc.nilSigs[recvMsg.ViewID][senderKeyStr]
	if ok {
		return errDupM2
	}
	if !recvMsg.ViewchangeSig.VerifyHash(senderKey.Object, NIL) {
		return errVerifyM2
	}

	vc.getLogger().Info().Uint64("viewID", recvMsg.ViewID).
		Str("validatorPubKey", senderKeyStr).
		Msg("[ProcessViewChangeMsg] Add M2 (NIL) type message")

	if _, ok := vc.nilSigs[recvMsg.ViewID]; !ok {
		vc.nilSigs[recvMsg.ViewID] = map[string]*bls_core.Sign{}
	}
	vc.nilSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewchangeSig

	if _, ok := vc.nilBitmap[recvMsg.ViewID]; !ok {
		nilBitmap := bls_cosi.NewMask(decider.Participants())
		vc.nilBitmap[recvMsg.ViewID] = nilBitmap
	}
	vc.nilBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true) // Set the bitmap indicating that this validator signed.

	vc.getLogger().Info().Uint64("viewID", recvMsg.ViewID).
		Str("validatorPubKey", senderKeyStr).
		Msg("[ProcessViewChangeMsg] Add M3 (ViewID) type message")

	if _, ok := vc.viewIDSigs[recvMsg.ViewID]; !ok {
		vc.viewIDSigs[recvMsg.ViewID] = map[string]*bls_core.Sign{}
	}
	vc.viewIDSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewidSig

	// Set the bitmap indicating that this validator signed.
	if _, ok := vc.viewIDBitmap[recvMsg.ViewID]; !ok {
		viewIDBitmap := bls_cosi.NewMask(decider.Participants())
		vc.viewIDBitmap[recvMsg.ViewID] = viewIDBitmap
	}
	vc.viewIDBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true)

	return nil
}

// InitPayload do it once when validator received view change message
func (vc *viewChange) InitPayload(
	fbftlog *FBFTLog,
	viewID uint64,
	blockNum uint64,
	privKeys multibls.PrivateKeys,
	members multibls.PublicKeys,
) error {
	// m1 or m2 init once per viewID/key.
	// m1 and m2 are mutually exclusive.
	// If the node has valid prepared block, it will add m1 signature.
	// If not, the node will add m2 signature.
	// Honest node should have only one kind of m1/m2 signature.

	inited := false
	for _, key := range privKeys {
		_, ok1 := vc.bhpSigs[viewID][key.Pub.Bytes.Hex()]
		_, ok2 := vc.nilSigs[viewID][key.Pub.Bytes.Hex()]
		if ok1 || ok2 {
			inited = true
			break
		}
	}

	// add my own M1/M2 type message signature and bitmap
	if !inited {
		preparedMsgs := fbftlog.GetMessagesByTypeSeq(msg_pb.MessageType_PREPARED, blockNum)
		preparedMsg := fbftlog.FindMessageByMaxViewID(preparedMsgs)
		hasBlock := false
		if preparedMsg != nil {
			if preparedBlock := fbftlog.GetBlockByHash(preparedMsg.BlockHash); preparedBlock != nil {
				if err := vc.verifyBlock(preparedBlock); err == nil {
					vc.getLogger().Info().Uint64("viewID", viewID).Uint64("blockNum", blockNum).Int("size", binary.Size(preparedBlock)).Msg("[InitPayload] add my M1 (prepared) type messaage")
					msgToSign := append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
					for _, key := range privKeys {
						// update the dictionary key if the viewID is first time received
						if _, ok := vc.bhpBitmap[viewID]; !ok {
							bhpBitmap := bls_cosi.NewMask(members)
							vc.bhpBitmap[viewID] = bhpBitmap
						}
						if err := vc.bhpBitmap[viewID].SetKey(key.Pub.Bytes, true); err != nil {
							vc.getLogger().Warn().Str("key", key.Pub.Bytes.Hex()).Msg("[InitPayload] bhpBitmap setkey failed")
							continue
						}
						if _, ok := vc.bhpSigs[viewID]; !ok {
							vc.bhpSigs[viewID] = map[string]*bls_core.Sign{}
						}
						vc.bhpSigs[viewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(msgToSign)
					}
					hasBlock = true
					// if m1Payload is empty, we just add one
					if vc.isM1PayloadEmpty() {
						vc.m1Payload = append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
					}
				}
			}
		}
		if !hasBlock {
			vc.getLogger().Info().Uint64("viewID", viewID).Uint64("blockNum", blockNum).Msg("[InitPayload] add my M2 (NIL) type messaage")
			for _, key := range privKeys {
				if _, ok := vc.nilBitmap[viewID]; !ok {
					nilBitmap := bls_cosi.NewMask(members)
					vc.nilBitmap[viewID] = nilBitmap
				}
				if err := vc.nilBitmap[viewID].SetKey(key.Pub.Bytes, true); err != nil {
					vc.getLogger().Warn().Err(err).
						Str("key", key.Pub.Bytes.Hex()).Msg("[InitPayload] nilBitmap setkey failed")
					continue
				}
				if _, ok := vc.nilSigs[viewID]; !ok {
					vc.nilSigs[viewID] = map[string]*bls_core.Sign{}
				}
				vc.nilSigs[viewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(NIL)
			}
		}
	}

	inited = false
	for _, key := range privKeys {
		_, ok3 := vc.viewIDSigs[viewID][key.Pub.Bytes.Hex()]
		if ok3 {
			inited = true
			break
		}
	}

	// add my own M3 type message signature and bitmap
	if !inited {
		viewIDBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(viewIDBytes, viewID)
		vc.getLogger().Info().Uint64("viewID", viewID).Uint64("blockNum", blockNum).Msg("[InitPayload] add my M3 (ViewID) type messaage")
		for _, key := range privKeys {
			if _, ok := vc.viewIDBitmap[viewID]; !ok {
				viewIDBitmap := bls_cosi.NewMask(members)
				vc.viewIDBitmap[viewID] = viewIDBitmap
			}
			if err := vc.viewIDBitmap[viewID].SetKey(key.Pub.Bytes, true); err != nil {
				vc.getLogger().Warn().Err(err).
					Str("key", key.Pub.Bytes.Hex()).Msg("[InitPayload] viewIDBitmap setkey failed")
				continue
			}
			if _, ok := vc.viewIDSigs[viewID]; !ok {
				vc.viewIDSigs[viewID] = map[string]*bls_core.Sign{}
			}
			vc.viewIDSigs[viewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(viewIDBytes)
		}
	}

	return nil
}

// isM1PayloadEmpty returns true if m1Payload is not set
// this is an unlocked internal function call
func (vc *viewChange) isM1PayloadEmpty() bool {
	return len(vc.m1Payload) == 0
}

// IsM1PayloadEmpty returns true if m1Payload is not set
func (vc *viewChange) IsM1PayloadEmpty() bool {
	return vc.isM1PayloadEmpty()
}

// GetViewIDBitmap returns the viewIDBitmap
func (vc *viewChange) GetViewIDBitmap(viewID uint64) *bls_cosi.Mask {
	return vc.viewIDBitmap[viewID]
}

// GetM1Payload returns the m1Payload.
func (vc *viewChange) GetM1Payload() []byte {
	return vc.m1Payload
}

// getLogger returns logger for view change contexts added
func (vc *viewChange) getLogger() *zerolog.Logger {
	logger := utils.Logger().With().
		Str("context", "ViewChange").
		Logger()
	return &logger
}
