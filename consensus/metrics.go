package consensus

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto/message"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	errSender  = "Invalid/Unknown sender"
	errMessage = "Invalid/Unknown message type"
)

var (
	defaultBytesDistribution        = view.Distribution(256, 512, 768, 1024, 1280, 1536, 1792, 2048, 2304, 2560, 2816, 3072)
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
		Name:        "ReceivedMessages",
		Measure:     ReceivedMessages,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: view.Count(),
	},
	&view.View{
		Name:        "ReceivedMessageErrors",
		Measure:     ReceivedMessageErrors,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey, KeyErrorMsg},
		Aggregation: view.Count(),
	},
	&view.View{
		Name:        "ReceivedBytes",
		Measure:     ReceivedBytes,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: defaultBytesDistribution,
	},
	&view.View{
		Name:        "InboundRequestLatency",
		Measure:     InboundRequestLatency,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: defaultMillisecondsDistribution,
	},
	&view.View{
		Name:        "OutboundRequestLatency",
		Measure:     OutboundRequestLatency,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: defaultMillisecondsDistribution,
	},
	&view.View{
		Name:        "SentMessages",
		Measure:     SentMessages,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: view.Count(),
	},
	&view.View{
		Name:        "SentMessageErrors",
		Measure:     SentMessageErrors,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey, KeyErrorMsg},
		Aggregation: view.Count(),
	},
	&view.View{
		Name:        "SentBytes",
		Measure:     SentBytes,
		TagKeys:     []tag.Key{KeyMessageType, KeyBlsKey},
		Aggregation: defaultBytesDistribution,
	},
}

type ConcensusMetrics struct {
	NumSent           []MessageDataPoint   `json:"sentMessages"`
	BytesSentHist     []HistogramDataPoint `json:"sentBytes"`
	NumSentErrors     []ErrorDataPoint     `json:"sentErrors"`
	NumReceived	      []MessageDataPoint   `json:"receivedMessages"`
	BytesReceivedHist []HistogramDataPoint `json:"receivedBytes"`
	NumReceivedErrors []ErrorDataPoint     `json:"receivedErrors"`
}

type MessageDataPoint struct {
	MessageType string `json:"messageType"`
	BlsKey      string `json:"blsKey"`
}

type ErrorDataPoint struct {
	MessageType  string `json:"messageType"`
	BlsKey       string `json:"blsKey"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type HistogramDataPoint struct {
	Bucket string `json:"bucket"`
	Value  string `json:"count"`
}

type printExporter struct {}

func (pe *printExporter) ExportView(vd *view.Data) {
	fmt.Println("Consensus Metrics")
	for i, row := range vd.Rows {
		fmt.Printf("\tRow: %#d: %#v\n", i, row)
	}
  fmt.Printf("StartTime: %s EndTime: %s\n\n", vd.Start.Round(0), vd.End.Round(0))
}

// Testing code && Remove before merge
func init() {
	if err := view.Register(ConsensusViews...); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register the views: %v\n", err)
		os.Exit(1)
	}
	view.RegisterExporter(new(printExporter))
	view.SetReportingPeriod(60 * time.Second)
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

func GetConsensusMetrics() {
	data := ConsensusMetrics{}
	for i, v := range ConsensusViews {
		if strings.HasSuffix(v.Name, "Latency") {
			continue
		} else if strings.HasSuffix(v.Name, "Bytes") {
			byteData := HistogramMetrics{}

		} else if strings.HasSuffix(v.Name, "Errors") {
			// Process Errors
		} else {
			messageData := []MessageDataPoint{}
			// Process data
			if strings.HasPrefix(v.Name, "Sent") {
				data.NumSent = messageData
			} else {
				data.NumReceived = messageData
			}
		}
	}
}
