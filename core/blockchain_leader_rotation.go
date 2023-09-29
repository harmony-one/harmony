package core

import (
	"bytes"
	"hash/crc32"
	"strconv"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
)

type LeaderRotationMeta struct {
	Pub    []byte
	Epoch  uint64
	Count  uint64
	Shifts uint64
}

// Hash returns has of the struct
func (a LeaderRotationMeta) Hash() []byte {
	c := crc32.NewIEEE()
	c.Write(a.Pub)
	c.Write([]byte(strconv.FormatUint(a.Epoch, 10)))
	c.Write([]byte(strconv.FormatUint(a.Count, 10)))
	c.Write([]byte(strconv.FormatUint(a.Shifts, 10)))
	return c.Sum(nil)
	//h := hash.C
	//h.
	// return
}

func (a LeaderRotationMeta) Clone() LeaderRotationMeta {
	return LeaderRotationMeta{
		Pub:    append([]byte{}, a.Pub...),
		Epoch:  a.Epoch,
		Count:  a.Count,
		Shifts: a.Shifts,
	}
}

// buildLeaderRotationMeta builds leader rotation meta if feature is activated.
func (bc *BlockChainImpl) buildLeaderRotationMeta(curHeader *block.Header) error {
	if !bc.chainConfig.IsLeaderRotationInternalValidators(curHeader.Epoch()) {
		return nil
	}
	if curHeader.NumberU64() == 0 {
		return errors.New("current header is genesis")
	}
	curPubKey, err := bc.getLeaderPubKeyFromCoinbase(curHeader)
	if err != nil {
		return err
	}
	for i := curHeader.NumberU64() - 1; i >= 0; i-- {
		header := bc.GetHeaderByNumber(i)
		if header == nil {
			return errors.New("header is nil")
		}
		blockPubKey, err := bc.getLeaderPubKeyFromCoinbase(header)
		if err != nil {
			return err
		}
		if curPubKey.Bytes != blockPubKey.Bytes || curHeader.Epoch().Uint64() != header.Epoch().Uint64() {
			for j := i; j <= curHeader.NumberU64(); j++ {
				header := bc.GetHeaderByNumber(j)
				if header == nil {
					return errors.New("header is nil")
				}
				err := bc.saveLeaderRotationMeta(header)
				if err != nil {
					utils.Logger().Error().Err(err).Msg("save leader continuous blocks count error")
					return err
				}
			}
			return nil
		}
	}
	return errors.New("no leader rotation meta to save")
}

func (bc *BlockChainImpl) saveLeaderRotationMeta(h *block.Header) error {
	blockPubKey, err := bc.getLeaderPubKeyFromCoinbase(h)
	if err != nil {
		return err
	}
	bc.leaderRotationMeta = processRotationMeta(h.Epoch().Uint64(), blockPubKey.Bytes, bc.leaderRotationMeta)
	return nil
}

func processRotationMeta(epoch uint64, blockPubKey bls.SerializedPublicKey, s LeaderRotationMeta) LeaderRotationMeta {
	// increase counter only if the same leader and epoch
	if bytes.Equal(s.Pub, blockPubKey[:]) && s.Epoch == epoch {
		s.Count++
	} else {
		s.Count = 1
	}
	// we should increase shifts if the leader has changed.
	if !bytes.Equal(s.Pub, blockPubKey[:]) {
		s.Shifts++
	}
	// but set to zero if new
	if s.Epoch != epoch {
		s.Shifts = 0
	}
	s.Epoch = epoch
	return LeaderRotationMeta{
		Pub:    blockPubKey[:],
		Epoch:  s.Epoch,
		Count:  s.Count,
		Shifts: s.Shifts,
	}
}
