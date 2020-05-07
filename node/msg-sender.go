package node

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	ggio "github.com/gogo/protobuf/io"
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/p2p"
	"github.com/libp2p/go-libp2p-core/helpers"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-msgio"
)

var (
	dhtReadMessageTimeout = time.Minute
	dhtStreamIdleTimeout  = 10 * time.Minute
	errReadTimeout        = fmt.Errorf("timed out reading response")
)

type senderWrapper struct {
	sync.Mutex
	strmap map[peer.ID]*messageSender
}

type messageSender struct {
	s         inet.Stream
	r         msgio.ReadCloser
	lk        sync.Mutex
	p         peer.ID
	host      *p2p.Host
	invalid   bool
	singleMes int
}

// invalidate is called before this messageSender is removed from the strmap.
// It prevents the messageSender from being reused/reinitialized and then
// forgotten (leaving the stream open).
func (ms *messageSender) invalidate() {
	ms.invalid = true
	if ms.s != nil {
		_ = ms.s.Reset()
		ms.s = nil
	}
}

func (ms *messageSender) prepOrInvalidate(ctx context.Context) error {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	if err := ms.prep(ctx); err != nil {
		ms.invalidate()
		return err
	}
	return nil
}

func (ms *messageSender) prep(ctx context.Context) error {
	if ms.invalid {
		return fmt.Errorf("message sender has been invalidated")
	}
	if ms.s != nil {
		return nil
	}

	nstr, err := ms.host.IPFSNode.PeerHost.NewStream(ctx, ms.p, p2p.Protocol)

	if err != nil {
		return err
	}

	ms.r = msgio.NewVarintReaderSize(nstr, inet.MessageSizeMax)
	ms.s = nstr

	return nil
}

// streamReuseTries is the number of times we will try to reuse a stream to a
// given peer before giving up and reverting to the old one-message-per-stream
// behaviour.
const streamReuseTries = 3

func (ms *messageSender) SendMessage(
	ctx context.Context, pmes *msg_pb.Message,
) error {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	retry := false
	for {
		if err := ms.prep(ctx); err != nil {
			return err
		}

		if err := ms.writeMsg(pmes); err != nil {
			_ = ms.s.Reset()
			ms.s = nil

			if retry {
				//log.Info("error writing message, bailing: ", err)
				return err
			}
			//log.Info("error writing message, trying again: ", err)
			retry = true
			continue
		}

		if ms.singleMes > streamReuseTries {
			go helpers.FullClose(ms.s)
			ms.s = nil
		} else if retry {
			ms.singleMes++
		}

		return nil
	}
}

func (ms *messageSender) SendRequest(
	ctx context.Context, pmes *msg_pb.Message,
) (*msg_pb.Message, error) {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	retry := false
	for {
		if err := ms.prep(ctx); err != nil {
			return nil, err
		}

		if err := ms.writeMsg(pmes); err != nil {
			_ = ms.s.Reset()
			ms.s = nil

			if retry {
				//log.Info("error writing message, bailing: ", err)
				return nil, err
			}
			//log.Info("error writing message, trying again: ", err)
			retry = true
			continue
		}

		mes := new(msg_pb.Message)
		if err := ms.ctxReadMsg(ctx, mes); err != nil {
			_ = ms.s.Reset()
			ms.s = nil

			if retry {
				//log.Info("error reading message, bailing: ", err)
				return nil, err
			}
			//log.Info("error reading message, trying again: ", err)
			retry = true
			continue
		}

		if ms.singleMes > streamReuseTries {
			go helpers.FullClose(ms.s)
			ms.s = nil
		} else if retry {
			ms.singleMes++
		}

		return mes, nil
	}
}

func (ms *messageSender) writeMsg(pmes *msg_pb.Message) error {
	return writeMsg(ms.s, pmes)
}

func (ms *messageSender) ctxReadMsg(
	ctx context.Context, mes *msg_pb.Message,
) error {
	errc := make(chan error, 1)
	go func(r msgio.ReadCloser) {
		bytes, err := r.ReadMsg()
		defer r.ReleaseMsg(bytes)
		if err != nil {
			errc <- err
			return
		}
		errc <- protobuf.Unmarshal(bytes, mes)
	}(ms.r)

	t := time.NewTimer(dhtReadMessageTimeout)
	defer t.Stop()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return errReadTimeout
	}
}

type bufferedDelimitedWriter struct {
	*bufio.Writer
	ggio.WriteCloser
}

func (w *bufferedDelimitedWriter) Flush() error {
	return w.Writer.Flush()
}

var writerPool = sync.Pool{
	New: func() interface{} {
		w := bufio.NewWriter(nil)
		return &bufferedDelimitedWriter{
			Writer:      w,
			WriteCloser: ggio.NewDelimitedWriter(w),
		}
	},
}

func writeMsg(w io.Writer, mes *msg_pb.Message) error {
	bw := writerPool.Get().(*bufferedDelimitedWriter)
	bw.Reset(w)
	err := bw.WriteMsg(mes)
	if err == nil {
		err = bw.Flush()
	}
	bw.Reset(nil)
	writerPool.Put(bw)
	return err
}
