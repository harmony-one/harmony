package consensus

import (
  "container/ring"
  "encoding/json"
  "sync"
  "time"

  protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto/message"
  "github.com/harmony-one/harmony/crypto/bls"
)

// Ring buffer size
const bufferSize = 1024

type MessageLog struct {
  MessageType   string `json:"messageType"`
  MessageSender string `json:"senderBlsKey"`
  CurrentLeader string `json:"leaderBlsKey,omitempty"`
  MessageSize   int    `json:"messageSize"`
  ErrorMessage  string `json:"errorMessage,omitempty"`
  ShardID       uint32 `json:"shardID"`
  ViewID        uint64 `json:"viewID"`
  BlockNum      uint64 `json:"blockHeight"`
  BlockHash     string `json:"blockHash,omitempty"`
  Timestamp     string `json:"timestamp"` //Timestamp at which finished being processed
  Received      bool   `json:"receivedMessage,omitempty"`
}

// Consensus message log, including errors, saving in memory
type ConsensusLog struct {
  Log *ring.Ring
  sync.Mutex
}

var (
  cLog           ConsensusLog
  // TODO: Come back to this error
  errParsePubKey = "Unable to parse bls key"
  timeFormat     = "Jan 2 15:04:05 2006 UTC"
)

func init() {
  cLog = ConsensusLog{}
  cLog.Log = ring.New(bufferSize)
}

// errMsg = "", means no errors
func addToConsensusLog(msg *message.Message, errMsg string) {
  msgToAdd := MessageLog{}
  // If message is nil, then failed to unmarshal message
  if (msg == nil) {
    msgToAdd.ErrorMessage = errMsg
    msgToAdd.Timestamp = getFormattedTime()
  } else if (msg.Type == message.MessageType_VIEWCHANGE || msg.Type == message.MessageType_NEWVIEW) {
    // View change messages
    viewChangeMessage := msg.GetViewchange()
    msgToAdd = MessageLog{
      msg.Type.String(),
      getKeyAsString(viewChangeMessage.GetSenderPubkey()),
      getKeyAsString(viewChangeMessage.GetLeaderPubkey()),
      int(protobuf.Size(msg)),
      errMsg,
      viewChangeMessage.GetShardId(),
      viewChangeMessage.GetViewId(),
      viewChangeMessage.GetBlockNum(),
      "", // No block hash for ViewChange messages
      getFormattedTime(),
      false,
    }
  } else {
    // Other consensus messages
    consensusMessage := msg.GetConsensus()
    msgToAdd = MessageLog{
      msg.Type.String(),
      getKeyAsString(consensusMessage.GetSenderPubkey()),
      "", // TODO: Come back to this, Leader not propagated in Consensus messages
      int(protobuf.Size(msg)),
      errMsg,
      consensusMessage.GetShardId(),
      consensusMessage.GetViewId(),
      consensusMessage.GetBlockNum(),
      string(consensusMessage.GetBlockHash()),
      getFormattedTime(),
      false,
    }
  }
  // Lock & add to ring buffer
  cLog.Lock()
  defer cLog.Unlock()
  cLog.Log.Value = msgToAdd
  cLog.Log = cLog.Log.Next()
  return
}

func getKeyAsString(key []byte) string {
  pubKey, err := bls.BytesToBlsPublicKey(key)
  if err != nil {
    return errParsePubKey //TODO: Come back to this
  }
  return pubKey.SerializeToHexStr()
}

func getFormattedTime() string {
  return time.Now().Format(timeFormat)
}

// Return JSON for ring buffer
func GetRawMetrics() []byte {
  rawMetrics := []MessageLog{}
  cLog.Lock()
  defer cLog.Unlock()
  cLog.Log.Do(func (v interface{}) {
    rawMetrics = append(rawMetrics, v.(MessageLog))
  })
  out, err := json.Marshal(rawMetrics)
  if err != nil {
    return []byte{} //TODO: Come back to this
  }
  return out
}
