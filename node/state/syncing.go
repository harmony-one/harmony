package state

type Syncer interface {
}

type syncer struct {
	Syncer
}

func NewSyncer() Syncer {
	return &syncer{}
}
