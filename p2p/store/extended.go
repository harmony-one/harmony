package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/common/clock"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

type extendedStore struct {
	peerstore.Peerstore
	peerstore.CertifiedAddrBook
	*scoreBook
	*peerBanBook
	*ipBanBook
	*metadataBook
}

func NewExtendedPeerstore(ctx context.Context, logger log.Logger, clock clock.Clock, ps peerstore.Peerstore, store ds.Batching, scoreRetention time.Duration) (ExtendedPeerstore, error) {
	cab, ok := peerstore.GetCertifiedAddrBook(ps)
	if !ok {
		return nil, errors.New("peerstore should also be a certified address book")
	}
	sb, err := newScoreBook(ctx, logger, clock, store, scoreRetention)
	if err != nil {
		return nil, fmt.Errorf("create scorebook: %w", err)
	}
	sb.startGC()
	pb, err := newPeerBanBook(ctx, logger, clock, store)
	if err != nil {
		return nil, fmt.Errorf("create peer ban book: %w", err)
	}
	pb.startGC()
	ib, err := newIPBanBook(ctx, logger, clock, store)
	if err != nil {
		return nil, fmt.Errorf("create IP ban book: %w", err)
	}
	ib.startGC()
	md, err := newMetadataBook(ctx, logger, clock, store)
	if err != nil {
		return nil, fmt.Errorf("create metadata book: %w", err)
	}
	md.startGC()
	return &extendedStore{
		Peerstore:         ps,
		CertifiedAddrBook: cab,
		scoreBook:         sb,
		peerBanBook:       pb,
		ipBanBook:         ib,
		metadataBook:      md,
	}, nil
}

func (s *extendedStore) Close() error {
	s.scoreBook.Close()
	s.peerBanBook.Close()
	s.ipBanBook.Close()
	s.metadataBook.Close()
	return s.Peerstore.Close()
}

var _ ExtendedPeerstore = (*extendedStore)(nil)
