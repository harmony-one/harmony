package knownpeers

import "github.com/harmony-one/harmony/p2p"

type Noop struct {
}

func (n Noop) GetChecked(limit int) []p2p.Peer {
	return nil
}

func (n Noop) GetUnchecked(limit int) []p2p.Peer {
	return nil
}

func (n Noop) AddChecked(hosts ...p2p.Peer) {
	return
}

func (n Noop) AddUnchecked(hosts ...p2p.Peer) {
	return
}
