package p2ptypes

// LifeCycle is the interface of module supports Start and Close
type LifeCycle interface {
	Start()
	Close()
}
