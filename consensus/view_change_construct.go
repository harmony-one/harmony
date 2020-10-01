package consensus

import (
	"bytes"
	"encoding/binary"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
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

// ViewChangeMsgType defines the type of view change message
type ViewChangeMsgType byte

const (
	// M1 is the view change message with block
	M1 ViewChangeMsgType = iota
	// M2 is the view change message w/o block
	M2
	// ERR is the error condition when parsing the message type
	ERR

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
	viewIDCount  map[uint64]map[string]int
	newViewMsg   map[uint64]map[string]uint64

	m1Payload []byte // message payload for type m1 := |vcBlockHash|prepared_agg_sigs|prepared_bitmap|, new leader only need one

	blockVerifier      BlockVerifierFunc
	viewChangeDuration time.Duration

	// mutex for view change
	vcLock sync.RWMutex
}

// NewViewChange returns a new viewChange object
func NewViewChange() *viewChange {
	vc := viewChange{}
	vc.Reset()
	return &vc
}

// SetBlockVerifier ..
func (vc *viewChange) SetBlockVerifier(verifier BlockVerifierFunc) {
	vc.blockVerifier = verifier
}

// AddViewIDKeyIfNotExist ..
func (vc *viewChange) AddViewIDKeyIfNotExist(viewID uint64, members multibls.PublicKeys) {
	vc.vcLock.Lock()
	defer vc.vcLock.Unlock()
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
		bhpBitmap, _ := bls_cosi.NewMask(members, nil)
		vc.bhpBitmap[viewID] = bhpBitmap
	}
	if _, ok := vc.nilBitmap[viewID]; !ok {
		nilBitmap, _ := bls_cosi.NewMask(members, nil)
		vc.nilBitmap[viewID] = nilBitmap
	}
	if _, ok := vc.viewIDBitmap[viewID]; !ok {
		viewIDBitmap, _ := bls_cosi.NewMask(members, nil)
		vc.viewIDBitmap[viewID] = viewIDBitmap
	}
}

// Reset reset the state for viewchange
func (vc *viewChange) Reset() {
	vc.vcLock.Lock()
	defer vc.vcLock.Unlock()

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
func (vc *viewChange) GetPreparedBlock(fbftlog *FBFTLog, hash [32]byte) ([]byte, []byte) {
	vc.vcLock.RLock()
	defer vc.vcLock.RUnlock()

	if !vc.IsM1PayloadEmpty() {
		block := fbftlog.GetBlockByHash(hash)
		if block != nil {
			encodedBlock, err := rlp.EncodeToBytes(block)
			if err != nil || len(encodedBlock) == 0 {
				vc.getLogger().Err(err).Msg("[GetPreparedBlock] Failed encoding prepared block")
				return vc.m1Payload, nil
			}
			return vc.m1Payload, encodedBlock
		}
	}
	return vc.m1Payload, nil
}

// GetM2Bitmap returns the nilBitmap as M2Bitmap
func (vc *viewChange) GetM2Bitmap(viewID uint64) ([]byte, []byte) {
	vc.vcLock.RLock()
	defer vc.vcLock.RUnlock()

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
	vc.vcLock.RLock()
	defer vc.vcLock.RUnlock()

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
	vc.vcLock.Lock()
	defer vc.vcLock.Unlock()

	if recvMsg.M3AggSig == nil || recvMsg.M3Bitmap == nil {
		return nil, errors.New("[VerifyNewViewMsg] M3AggSig or M3Bitmap is nil")
	}

	senderKey := recvMsg.SenderPubkey
	senderKeyStr := senderKey.Bytes.Hex()

	// first time received the new view message
	_, ok := vc.newViewMsg[recvMsg.ViewID]
	if !ok {
		newViewMap := map[string]uint64{}
		vc.newViewMsg[recvMsg.ViewID] = newViewMap
	}

	_, ok = vc.newViewMsg[recvMsg.ViewID][senderKeyStr]
	if ok {
		return nil, errors.New("[VerifyNewViewMsg] received new view message already")
	}

	vc.newViewMsg[recvMsg.ViewID][senderKeyStr] = recvMsg.BlockNum

	m3Sig := recvMsg.M3AggSig
	m3Mask := recvMsg.M3Bitmap

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)

	if !m3Sig.VerifyHash(m3Mask.AggregatePublic, viewIDBytes) {
		return nil, errors.New("[VerifyNewViewMsg] Unable to Verify Aggregated Signature of M3 (ViewID) payload")
	}

	m2Mask := recvMsg.M2Bitmap
	if recvMsg.M2AggSig != nil {
		m2Sig := recvMsg.M2AggSig
		if !m2Sig.VerifyHash(m2Mask.AggregatePublic, NIL) {
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
		if err := vc.blockVerifier(preparedBlock); err != nil {
			return nil, errors.New("[VerifyNewViewMsg] Prepared block verification failed")
		}
		return preparedBlock, nil
	}
	return nil, nil
}

// VerifyViewChangeMsg returns the type of the view change message after verification
func (vc *viewChange) VerifyViewChangeMsg(recvMsg *FBFTMessage, members multibls.PublicKeys) (ViewChangeMsgType, *types.Block, error) {
	vc.vcLock.Lock()
	defer vc.vcLock.Unlock()

	preparedBlock := &types.Block{}
	senderKey := recvMsg.SenderPubkey
	senderKeyStr := senderKey.Bytes.Hex()

	// check and add viewID (m3 type) message signature
	if _, ok := vc.viewIDSigs[recvMsg.ViewID][senderKeyStr]; ok {
		return ERR, nil, errors.New("[VerifyViewChangeMsg] received M3 (ViewID) message already")
	}

	if len(recvMsg.Payload) >= ValidPayloadLength && len(recvMsg.Block) != 0 {
		if err := rlp.DecodeBytes(recvMsg.Block, preparedBlock); err != nil {
			return ERR, nil, err
		}
		if err := vc.blockVerifier(preparedBlock); err != nil {
			return ERR, nil, err
		}
		_, ok := vc.bhpSigs[recvMsg.ViewID][senderKeyStr]
		if ok {
			return M1, nil, errors.New("[VerifyViewChangeMsg] received M1 (prepared) message already")
		}
		if !recvMsg.ViewchangeSig.VerifyHash(senderKey.Object, recvMsg.Payload) {
			return ERR, nil, errors.New("[VerifyViewChangeMsg] failed to verify signature for m1 type message")
		}
		blockHash := recvMsg.Payload[:32]

		aggSig, mask, err := chain.ReadSignatureBitmapByPublicKeys(recvMsg.Payload[32:], members)
		if err != nil {
			return ERR, nil, err
		}
		if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
			return ERR, nil, errors.New("[VerifyViewChangeMsg] failed to verify multi signature for m1 prepared payload")
		}

		vc.getLogger().Info().Uint64("viewID", recvMsg.ViewID).
			Str("validatorPubKey", senderKeyStr).
			Msg("[VerifyViewChangeMsg] Add M1 (prepared) type message")
		vc.bhpSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewchangeSig
		vc.bhpBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true) // Set the bitmap indicating that this validator signed.

		vc.getLogger().Info().Uint64("viewID", recvMsg.ViewID).
			Str("validatorPubKey", senderKeyStr).
			Msg("[VerifyViewChangeMsg] Add M3 (ViewID) type message")

		vc.viewIDSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewidSig
		// Set the bitmap indicating that this validator signed.
		vc.viewIDBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true)

		return M1, preparedBlock, nil
	}
	_, ok := vc.nilSigs[recvMsg.ViewID][senderKeyStr]
	if ok {
		return M2, nil, errors.New("[VerifyViewChangeMsg] received M2 (NIL) message already")
	}
	if !recvMsg.ViewchangeSig.VerifyHash(senderKey.Object, NIL) {
		return ERR, nil, errors.New("[VerifyViewChangeMsg] failed to verify signature for m2 type message")
	}

	vc.getLogger().Info().Uint64("viewID", recvMsg.ViewID).
		Str("validatorPubKey", senderKeyStr).
		Msg("[VerifyViewChangeMsg] Add M2 (NIL) type message")
	vc.nilSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewchangeSig
	vc.nilBitmap[recvMsg.ViewID].SetKey(recvMsg.SenderPubkey.Bytes, true) // Set the bitmap indicating that this validator signed.

	vc.getLogger().Info().Uint64("viewID", recvMsg.ViewID).
		Str("validatorPubKey", senderKeyStr).
		Msg("[VerifyViewChangeMsg] Add M3 (ViewID) type message")

	vc.viewIDSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewidSig
	// Set the bitmap indicating that this validator signed.
	vc.viewIDBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true)

	return M2, nil, nil
}

// InitPayload do it once when validator received view change message
// TODO: if I start the view change message, I should init my payload already
func (vc *viewChange) InitPayload(
	fbftlog *FBFTLog,
	viewID uint64,
	blockNum uint64,
	myPubKey string,
	privKeys multibls.PrivateKeys,
) error {
	vc.vcLock.Lock()
	defer vc.vcLock.Unlock()

	// add my own M2 type message signature and bitmap
	_, ok := vc.nilSigs[viewID][myPubKey]

	if !ok {
		vc.getLogger().Info().Uint64("viewID", viewID).Uint64("blockNum", blockNum).Msg("[InitPayload] add my M2 (NIL) type messaage")
		for i, key := range privKeys {
			if err := vc.nilBitmap[viewID].SetKey(key.Pub.Bytes, true); err != nil {
				vc.getLogger().Warn().Int("index", i).Msg("[InitPayload] nilBitmap setkey failed")
				continue
			}
			vc.nilSigs[viewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(NIL)
		}
	}

	// add my own M3 type message signature and bitmap
	_, ok = vc.viewIDSigs[viewID][myPubKey]

	if !ok {
		viewIDBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(viewIDBytes, viewID)
		vc.getLogger().Info().Uint64("viewID", viewID).Uint64("blockNum", blockNum).Msg("[InitPayload] add my M3 (ViewID) type messaage")
		for i, key := range privKeys {
			if err := vc.viewIDBitmap[viewID].SetKey(key.Pub.Bytes, true); err != nil {
				vc.getLogger().Warn().Int("index", i).Msg("[InitPayload] viewIDBitmap setkey failed")
				continue
			}
			vc.viewIDSigs[viewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(viewIDBytes)
		}
	}

	// add my own M1 type message signature and bitmap
	_, ok = vc.bhpSigs[viewID][myPubKey]

	if !ok {
		preparedMsgs := fbftlog.GetMessagesByTypeSeq(msg_pb.MessageType_PREPARED, blockNum)
		preparedMsg := fbftlog.FindMessageByMaxViewID(preparedMsgs)
		if preparedMsg != nil {
			if preparedBlock := fbftlog.GetBlockByHash(preparedMsg.BlockHash); preparedBlock != nil {
				if err := vc.blockVerifier(preparedBlock); err != nil {
					return err
				}
				vc.getLogger().Info().Uint64("viewID", viewID).Uint64("blockNum", blockNum).Msg("[InitPayload] add my M1 (prepared) type messaage")
				msgToSign := append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
				for i, key := range privKeys {
					if err := vc.bhpBitmap[viewID].SetKey(key.Pub.Bytes, true); err != nil {
						vc.getLogger().Warn().Int("index", i).Msg("[InitPayload] bhpBitmap setkey failed")
						continue
					}
					vc.bhpSigs[viewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(msgToSign)
				}
				// if m1Payload is empty, we just add one
				if vc.IsM1PayloadEmpty() {
					vc.m1Payload = append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
				}
			}
		}
	}
	return nil
}

// IsM1PayloadEmpty returns true if m1Payload is not set
func (vc *viewChange) IsM1PayloadEmpty() bool {
	return len(vc.m1Payload) == 0
}

// GetViewIDBitmap returns the viewIDBitmap
func (vc *viewChange) GetViewIDBitmap(viewID uint64) *bls_cosi.Mask {
	vc.vcLock.RLock()
	defer vc.vcLock.RUnlock()
	return vc.viewIDBitmap[viewID]
}

// GetM1Payload returns the m1Payload
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
