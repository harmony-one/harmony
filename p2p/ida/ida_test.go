package ida

import (
	"math/rand"
	"testing"
	"time"

	"github.com/simple-rules/harmony-benchmark/p2p"
)

var (
	a = Symbol{0, 0}
	b = Symbol{1, 0}
	c = Symbol{0, 1}
	d = Symbol{1, 1}
)

type FakeRaptor struct {
	symbols []Symbol
	done    chan struct{}
	r       *rand.Rand
}

func (raptor *FakeRaptor) Init() {
	// raptor.symbols = make([]Symbol, 4)
	raptor.symbols = make([]Symbol, 4)
	raptor.symbols[0] = make(Symbol, 2)
	copy(raptor.symbols[0], a)
	raptor.symbols[1] = make(Symbol, 2)
	copy(raptor.symbols[1], b)
	raptor.symbols[2] = make(Symbol, 2)
	copy(raptor.symbols[2], c)
	raptor.symbols[3] = make(Symbol, 2)
	copy(raptor.symbols[3], d)
	raptor.r = rand.New(rand.NewSource(99))
	raptor.done = make(chan struct{})
}

func (raptor *FakeRaptor) generate(res chan Symbol) {
	for {
		select {
		case <-raptor.done:
			return
		default:
			i := raptor.r.Intn(4)
			res <- raptor.symbols[i]
		}
	}
}

func (raptor *FakeRaptor) Process(msg Message) chan Symbol {
	res := make(chan Symbol)
	go raptor.generate(res)
	return res
}

func TestShouldReturnErrRaptorImpNotFound(t *testing.T) {
	ida := &IDAImp{}
	done := make(chan struct{})
	err := ida.Process([]byte{}, []p2p.Peer{}, done, time.Second)
	if err != ErrRaptorImpNotFound {
		t.Fatal("Should return an error")
	}
}

func TestSimple(t *testing.T) {
	raptor := &FakeRaptor{}
	raptor.Init()

	// ida := &IDAImp{}
	// done := make(chan struct{})
	// ida.TakeRaptorQ(raptor)
	// err := ida.Process([]byte{}, []p2p.Peer{}, done, time.Second)
	// if err == ErrRaptorImpNotFound {
	// 	t.Fatal("Should return an error")
	// }
}
