package consensus

import (
	"encoding/binary"
	"sync"

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
	// M3 is the view change message with viewID ??
	M3
	// Err is the error condition when parsing the message type
	ERR
)

// TODO: move all viewID related function/data here
// ViewChange encapsulate all the view change related data structure and functions
type ViewChange struct {
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
	m1Payload    []byte // message payload for type m1 := |vcBlockHash|prepared_agg_sigs|prepared_bitmap|, new leader only need one

	blockVerifier types.BlockVerifier

	// mutex for view change
	vcLock sync.RWMutex
}

func NewViewChange() *ViewChange {
	vc := ViewChange{}
	vc.Reset()
	return &vc
}

// SetBlockVerifier ..
func (vc *ViewChange) SetBlockVerifier(verifier types.BlockVerifier) {
	vc.blockVerifier = verifier
}

// GetViewIDSigsArray returns the signatures for viewID in viewchange
func (vc *ViewChange) GetViewIDSigsArray(viewID uint64) []*bls_core.Sign {
	sigs := []*bls_core.Sign{}
	for _, sig := range vc.viewIDSigs[viewID] {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetNilSigsArray returns the signatures for nil prepared message in viewchange
func (vc *ViewChange) GetNilSigsArray(viewID uint64) []*bls_core.Sign {
	sigs := []*bls_core.Sign{}
	for _, sig := range vc.nilSigs[viewID] {
		sigs = append(sigs, sig)
	}
	return sigs
}

// AddViewIDKeyIfNotExist ..
func (vc *ViewChange) AddViewIDKeyIfNotExist(viewID uint64, members multibls.PublicKeys) {
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
func (vc *ViewChange) Reset() {
	vc.vcLock.Lock()
	defer vc.vcLock.Unlock()

	vc.m1Payload = []byte{}
	vc.bhpSigs = map[uint64]map[string]*bls_core.Sign{}
	vc.nilSigs = map[uint64]map[string]*bls_core.Sign{}
	vc.viewIDSigs = map[uint64]map[string]*bls_core.Sign{}
	vc.bhpBitmap = map[uint64]*bls_cosi.Mask{}
	vc.nilBitmap = map[uint64]*bls_cosi.Mask{}
	vc.viewIDBitmap = map[uint64]*bls_cosi.Mask{}
}

// GetPreparedBlock returns the prepared block or nil if not found
func (vc *ViewChange) GetPreparedBlock(fbftlog *FBFTLog, hash [32]byte) ([]byte, []byte) {
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
func (vc *ViewChange) GetM2Bitmap(viewID uint64) ([]byte, []byte) {
	vc.vcLock.RLock()
	defer vc.vcLock.RUnlock()

	sig2arr := vc.GetNilSigsArray(viewID)
	if len(sig2arr) > 0 {
		m2Sig := bls_cosi.AggregateSig(sig2arr)
		vc.getLogger().Info().Int("len", len(sig2arr)).Msg("[GetM2Bitmap] M2 (NIL) type signatures")
		return m2Sig.Serialize(), vc.nilBitmap[viewID].Bitmap
	}
	vc.getLogger().Info().Uint64("viewID", viewID).Msg("[GetM2Bitmap] No M2 (NIL) type signatures")
	return nil, nil
}

// GetM3Bitmap returns the viewIDBitmap as M3Bitmap
func (vc *ViewChange) GetM3Bitmap(viewID uint64) ([]byte, []byte) {
	vc.vcLock.RLock()
	defer vc.vcLock.RUnlock()

	sig3arr := vc.GetViewIDSigsArray(viewID)
	// even we check here for safty, m3 type signatures must >= 2f+1
	if len(sig3arr) > 0 {
		m3Sig := bls_cosi.AggregateSig(sig3arr)
		vc.getLogger().Info().Int("len", len(sig3arr)).Msg("[GetM2Bitmap] M3 (ViewID) type signatures")
		return m3Sig.Serialize(), vc.viewIDBitmap[viewID].Bitmap
	}
	vc.getLogger().Info().Uint64("viewID", viewID).Msg("[GetM3Bitmap] No M3 (ViewID) type signatures")
	return nil, nil
}

// VerifyViewChangeMsg returns the type of the view change message after verification
func (vc *ViewChange) VerifyViewChangeMsg(recvMsg *FBFTMessage, members multibls.PublicKeys) (ViewChangeMsgType, *types.Block, error) {
	vc.vcLock.Lock()
	defer vc.vcLock.Unlock()

	preparedBlock := &types.Block{}
	senderKey := recvMsg.SenderPubkey
	senderKeyStr := senderKey.Bytes.Hex()

	if len(recvMsg.Payload) != 0 && len(recvMsg.Block) != 0 {
		if err := rlp.DecodeBytes(recvMsg.Block, preparedBlock); err != nil {
			return ERR, nil, err
		}
		if err := vc.blockVerifier(preparedBlock); err != nil {
			return ERR, nil, err
		}
		if len(recvMsg.Payload) <= 32 {
			return M1, nil, errors.New("[VerifyViewChangeMsg] view change message payload is not 32 bytes")
		}
		_, ok := vc.bhpSigs[recvMsg.ViewID][senderKeyStr]
		if ok {
			return M1, nil, errors.New("[VerifyViewChangeMsg] received M1 message already")
		}
		if !recvMsg.ViewchangeSig.VerifyHash(senderKey.Object, recvMsg.Payload) {
			return ERR, nil, errors.New("[VerifyViewChangeMsg] failed to verify signature for m1 type message")
		}
		blockHash := recvMsg.Payload[:32]

		if 32+bls.BLSSignatureSizeInBytes > len(recvMsg.Payload) {
			return ERR, nil, errors.New("payload not have enough length")
		}
		aggSig, mask, err := chain.ReadSignatureBitmapByPublicKeys(recvMsg.Payload[32:], members)
		if err != nil {
			return ERR, nil, err
		}
		if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
			return ERR, nil, errors.New("[VerifyViewChangeMsg] failed to verify multi signature for m1 prepared payload")
		}

		vc.getLogger().Info().Str("validatorPubKey", senderKeyStr).
			Msg("[VerifyViewChangeMsg] Add M1 (prepared) type message")
		vc.bhpSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewchangeSig
		vc.bhpBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true) // Set the bitmap indicating that this validator signed.

		return M1, preparedBlock, nil
	}
	_, ok := vc.nilSigs[recvMsg.ViewID][senderKeyStr]
	if ok {
		return M2, nil, errors.New("[VerifyViewChangeMsg] received M2 message already")
	}
	if !recvMsg.ViewchangeSig.VerifyHash(senderKey.Object, NIL) {
		return ERR, nil, errors.New("[VerifyViewChangeMsg] failed to verify signature for m2 type message")
	}

	vc.getLogger().Info().Str("validatorPubKey", senderKeyStr).
		Msg("[VerifyViewChangeMsg] Add M2 (NIL) type message")
	vc.nilSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewchangeSig
	vc.nilBitmap[recvMsg.ViewID].SetKey(recvMsg.SenderPubkey.Bytes, true) // Set the bitmap indicating that this validator signed.

	// check and add viewID (m3 type) message signature
	if _, ok := vc.viewIDSigs[recvMsg.ViewID][senderKeyStr]; ok {
		vc.getLogger().Debug().
			Str("validatorPubKey", senderKeyStr).
			Msg("[onViewChange] Already Received M3(ViewID) message from the validator")
		return ERR, nil, errors.New("[VerifyViewChangeMsg] received M3 message already")
	}
	vc.getLogger().Info().Str("validatorPubKey", senderKeyStr).
		Msg("[VerifyViewChangeMsg] Add M3 (ViewID) type message")

	vc.viewIDSigs[recvMsg.ViewID][senderKeyStr] = recvMsg.ViewidSig
	// Set the bitmap indicating that this validator signed.
	vc.viewIDBitmap[recvMsg.ViewID].SetKey(senderKey.Bytes, true)

	return M2, nil, nil
}

// InitPayload do it once when validator received view change message
// TODO: if I start the view change message, I should init my payload already
func (vc *ViewChange) InitPayload(
	fbftlog *FBFTLog,
	viewID uint64,
	blockNum uint64,
	myPubKey string,
	privKeys multibls.PrivateKeys,
) error {
	vc.vcLock.Lock()
	defer vc.vcLock.Unlock()

	_, ok := vc.nilSigs[viewID][myPubKey]

	if !ok {
		vc.getLogger().Info().Msg("[InitPayload] add my M2(NIL) type messaage")
		for i, key := range privKeys {
			if err := vc.nilBitmap[viewID].SetKey(key.Pub.Bytes, true); err != nil {
				vc.getLogger().Warn().Int("index", i).Msg("[InitPayload] nilBitmap setkey failed")
				continue
			}
			vc.nilSigs[viewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(NIL)
		}
	}

	_, ok = vc.bhpSigs[viewID][myPubKey]

	if !ok {
		preparedMsgs := fbftlog.GetMessagesByTypeSeq(msg_pb.MessageType_PREPARED, blockNum)
		preparedMsg := fbftlog.FindMessageByMaxViewID(preparedMsgs)
		if preparedMsg != nil {
			if preparedBlock := fbftlog.GetBlockByHash(preparedMsg.BlockHash); preparedBlock != nil {
				if err := vc.blockVerifier(preparedBlock); err != nil {
					return err
				}
				vc.getLogger().Info().Msg("[InitPayload] add my M1 type messaage")
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

	// add self m3 type message signature and bitmap
	_, ok = vc.viewIDSigs[viewID][myPubKey]

	if !ok {
		viewIDBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(viewIDBytes, viewID)
		vc.getLogger().Info().Msg("[InitPayload] add my M3 type messaage")
		for i, key := range privKeys {
			if err := vc.viewIDBitmap[viewID].SetKey(key.Pub.Bytes, true); err != nil {
				vc.getLogger().Warn().Int("index", i).Msg("[InitPayload] viewIDBitmap setkey failed")
				continue
			}
			vc.viewIDSigs[viewID][key.Pub.Bytes.Hex()] = key.Pri.SignHash(viewIDBytes)
		}
	}
	return nil
}

// IsM1PayloadEmpty returns true if m1Payload is not set
func (vc *ViewChange) IsM1PayloadEmpty() bool {
	return len(vc.m1Payload) == 0
}

// GetViewIDBitmap returns the viewIDBitmap
func (vc *ViewChange) GetViewIDBitmap(viewID uint64) *bls_cosi.Mask {
	vc.vcLock.RLock()
	defer vc.vcLock.RUnlock()
	return vc.viewIDBitmap[viewID]
}

// GetM1Payload returns the m1Payload
func (vc *ViewChange) GetM1Payload() []byte {
	return vc.m1Payload
}

// getLogger returns logger for view change contexts added
func (vc *ViewChange) getLogger() *zerolog.Logger {
	logger := utils.Logger().With().
		Str("context", "ViewChange").
		Logger()
	return &logger
}
