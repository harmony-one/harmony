package consensus

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"contrib.go.opencensus.io/exporter/prometheus"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto/message"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	defaultBytesDistribution        = view.Distribution(1024, 2048, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216, 67108864, 268435456, 1073741824, 4294967296)
	defaultMillisecondsDistribution = view.Distribution(0.01, 0.05, 0.1, 0.3, 0.6, 0.8, 1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 50000, 100000)
)

// Keys
var (
	KeyMessageType, _ = tag.NewKey("message_type")
	KeyBlsKey, _      = tag.NewKey("bls_key")
	KeyErrorMsg, _    = tag.NewKey("error_msg")
)

// Measures
var (
	ReceivedMessages       = stats.Int64("consensus/received_messages", "Total number of messages received per RPC", stats.UnitDimensionless)
	ReceivedMessageErrors  = stats.Int64("consensus/received_message_errors", "Total number of errors for messages received per RPC", stats.UnitDimensionless)
	ReceivedBytes          = stats.Int64("consensus/received_bytes", "Total received bytes per RPC", stats.UnitBytes)
	InboundRequestLatency  = stats.Float64("consensus/inbound_request_latency", "Latency per RPC", stats.UnitMilliseconds)
	OutboundRequestLatency = stats.Float64("consensus/outbound_request_latency", "Latency per RPC", stats.UnitMilliseconds)
	SentMessages           = stats.Int64("consensus/sent_messages", "Total number of messages sent per RPC", stats.UnitDimensionless)
	SentMessageErrors      = stats.Int64("consensus/sent_message_errors", "Total number of errors for messages sent per RPC", stats.UnitDimensionless)
	SentBytes              = stats.Int64("consensus/sent_bytes", "Total sent bytes per RPC", stats.UnitBytes)
)

var ConsensusViews = []*view.View{
	&view.View{
		Measure:     ReceivedMessages,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: view.Count(),
	},
	&view.View{
		Measure:     ReceivedMessageErrors,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey, KeyErrorMsg},
		Aggregation: view.Count(),
	},
	&view.View{
		Measure:     ReceivedBytes,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: defaultBytesDistribution,
	},
	&view.View{
		Measure:     InboundRequestLatency,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: defaultMillisecondsDistribution,
	},
	&view.View{
		Measure:     OutboundRequestLatency,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: defaultMillisecondsDistribution,
	},
	&view.View{
		Measure:     SentMessages,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: view.Count(),
	},
	&view.View{
		Measure:     SentMessageErrors,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey, KeyErrorMsg},
		Aggregation: view.Count(),
	},
	&view.View{
		Measure:     SentBytes,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: defaultBytesDistribution,
	},
}

// Testing code && Remove before merge
func init() {
	if err := view.Register(ConsensusViews...); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register the views: %v\n", err)
		os.Exit(1)
	}
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "harmony",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create the Prometheus stats exporter: %v", err)
	}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", pe)
		if err := http.ListenAndServe(":8888", mux); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to run Prometheus scrape endpoint: %v", err)
		}
	}()
}

func incrementReceivedMessages(msg *message.Message, pubKey string) {
	msgType := msg.GetType().String()
	ctx, _ := tag.New(
		context.Background(),
		tag.Upsert(KeyMessageType, msgType),
	)

	stats.RecordWithTags(
		ctx,
		[]tag.Mutator{
			tag.Upsert(KeyMessageType, msgType),
			tag.Upsert(KeyBlsKey, pubKey),
		},
		ReceivedMessages.M(1),
		ReceivedBytes.M(int64(protobuf.Size(msg))),
	)
}

func incrementSentMessages(msg []byte, phase, pubKey string) {
	ctx, _ := tag.New(
		context.Background(),
		tag.Upsert(KeyMessageType, phase),
	)

	stats.RecordWithTags(
		ctx,
		[]tag.Mutator{
			tag.Upsert(KeyMessageType, phase),
			tag.Upsert(KeyBlsKey, pubKey),
		},
		SentMessages.M(1),
		SentBytes.M(int64(len(msg))),
	)
}

func incrementReceivedErrors(phase, pubKey string) {
	ctx, _ := tag.New(
		context.Background(),
		tag.Upsert(KeyMessageType, phase),
	)

	stats.RecordWithTags(
		ctx,
		[]tag.Mutator{
			tag.Upsert(KeyMessageType, phase),
			tag.Upsert(KeyBlsKey, pubKey),
		},
		ReceivedMessageErrors.M(1),
	)
}

func incrementSentErrors(phase, pubKey string) {
	ctx, _ := tag.New(
		context.Background(),
		tag.Upsert(KeyMessageType, phase),
	)

	stats.RecordWithTags(
		ctx,
		[]tag.Mutator{
			tag.Upsert(KeyMessageType, phase),
			tag.Upsert(KeyBlsKey, pubKey),
		},
		SentMessageErrors.M(1),
	)
}
