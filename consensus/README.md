Consensus package includes the Harmony BFT consensus protocol code, which uses BLS-based
multi-signature to cosign the new block. The details are
in Harmony's new [consensus protocol design](https://talk.harmony.one/t/bls-based-practical-bft-consensus/131).

## Introduction to Harmony BFT with BLS signatures

Harmony BFT consensus protocol consist of normal mode and view changing mode which is same
as the PBFT(practical byzantine fault tolerance) protocol. The difference is we use the
BLS aggregated signature to reduce O(N^2) communications to O(N), which is more efficient
and scalable to traditional PBFT. For brevity, we will still call the whole process as PBFT.

### Normal mode

To reach the consensus of the next block, there are 3 phases: announce(i.e. pre-prepare in PBFT), prepare and commit.

- Announce(leader): The leader broadcasts ANNOUNCE message along with candidate of the next block.
- Prepare(validator): The validator will validate the block sent by leader and send PREPARE message;
  if the block is invalid, the validator will propose view change. If the prepare timeout, the validator will also propose view change.
- Prepared(leader): The leader will collect 2f+1 PREPARE message including itself and broadcast PREPARED message with the aggregated signature
- Commit(validator): The validator will check the validity of aggregated signature (# of signatures >= 2f+1) and
  send COMMIT message; if the commit timeout, the validator will also propose view change.
- Committed(leader): The leader will collect 2f+1 COMMIT message including itself and broadcast COMMITTED message with the aggregated signature
- Finalize(leader and validators): Both the leader and validators will finalize the block into blockchain together with 2f+1 aggregated signatures.

### View changing mode

- ViewChange(validator): whenever a validator receives invalid block/signature from the leader,
  it should send VIEWCHANGE message with view v+1 together with its own prepared message(>=2f+1 aggregated prepare signatures) from previous views.
- NewView(new leader): when the new leader (uniquely determined) collect enough (2f+1) view change
  messages, it broadcasts the NEWVIEW message with aggregated VIEWCHANGE signatures.
- During the view changing process, if the new leader not send NEWVIEW message on time, the
  validator will propose ViewChange for the next view v+2 and so on...

## State Machine

The whole process of PBFT can be described as a state machine. We don't separate the roles of leader
and validators, instead we use PBFTState structure to describe the role and phase of a given node
who is joining the consensus process. When a node receives a new message from its peer, its state will be updated. i.e. pbft_state --(upon
receive new PBFTMessage)-->
new_pbft_state. Thus the most nature and clear way is to describe the whole process as state machine.

```golang
// PBFTState holds the state of a node in PBFT process
type PBFTState struct {
   IsLeader bool
   phase PBFTPhase // Announce, Prepare(d), Commit(ted)
   ...

}

// PBFTLog stores the data in PBFT process, it will be used in different phases in order to determine whether a new PBFTMessage is valid or not.
type PBFTLog struct {
    blocks []*types.Block
    messages []*PBFTMessage
}

// entry point and main loop;
// in each loop, the node will receive PBFT message from peers with timeout,
// then update its state accordingly. handleMessageUpdate function handles various kinds of messages and update its state
// it will also send new PBFT message (if not null) to its peers.
// in the same loop, the node will also check whether it should send view changing message to new leader
// finally, it will periodically try to publish new block into blockchain
func (consensus *Consensus) Start(stopChan chan struct{}, stoppedChan chan struct{}) {
    defer close(stoppedChan)
    tick := time.NewTicker(blockDuration)
    consensus.idleTimeout.Start()
    for {
        select {
        default:
            msg := consensus.recvWithTimeout(receiveTimeout)
            consensus.handleMessageUpdate(msg)
            if consensus.idleTimeout.CheckExpire() {
                consensus.startViewChange(consensus.viewID + 1)
            }
            if consensus.commitTimeout.CheckExpire() {
                consensus.startViewChange(consensus.viewID + 1)
            }
            if consensus.viewChangeTimeout.CheckExpire() {
                if consensus.mode.Mode() == Normal {
                    continue
                }
                viewID := consensus.mode.ViewID()
                consensus.startViewChange(viewID + 1)
            }
        case <-tick.C:
            consensus.tryPublishBlock()
        case <-stopChan:
            return
        }
    }

}
```
