package node

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
)

// ActionType ...
type ActionType byte

// Constants ...
const (
	SyncingAction ActionType = iota
	KillSyncingAction
	SupportClient
	SupportExplorer
	SyncingActionTest
	Done
)

const (
	// WaitForStatusUpdate is the delay time to update new status. Currently set 1 second for development. Should be 30 minutes for production.
	WaitForStatusUpdate = time.Second * 1
)

// Action is type of service action.
type Action struct {
	t      ActionType
	params map[string]interface{}
}

// Start node.
func (node *Node) Start() {
	node.actionChannel = node.StartServiceManager()
}

// SendAction ...
func (node *Node) SendAction(action *Action) {
	node.actionChannel <- action
}

// TakeAction ...
func (node *Node) TakeAction(action *Action) {
	switch action.t {
	case SyncingActionTest:
		fmt.Println("Running syncing support")
	case SyncingAction:
		utils.GetLogInstance().Info("Running syncing support")
		go node.SupportSyncing()
	case KillSyncingAction:
		utils.GetLogInstance().Info("Killing syncing")
	case SupportClient:
		go node.SupportClient()
	case SupportExplorer:
		go node.SupportExplorer()
	}
}

// StartServiceManager ...
func (node *Node) StartServiceManager() chan *Action {
	ch := make(chan *Action)
	go func() {
		for {
			select {
			case action := <-ch:
				node.TakeAction(action)
				if action.t == Done {
					return
				}
			case <-time.After(WaitForStatusUpdate):
				utils.GetLogInstance().Info("Waiting for new action.")
			}
		}
	}()
	return ch
}
