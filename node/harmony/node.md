## Node Architecture (WIP)

### Services

In Harmony network, a node can be treated as one of the roles: validator, leader, beacon validator,
beacon leader depending on its context. With each role, a node can run a certain set of services.

For example, a leader needs to run explorer support service, syncing support
service etc.. while a normal validator does not run such many.

### Service Manager

To support such behavior, we architecture Node logic with service manager which can wait for actions
which each triggers its management operation such as starting some service, stopping some service.

Each service needs to implement minimal interface behavior like Start, Stop so that the service
manager can handle those operations.

```go
// ServiceInterface is the collection of functions any service needs to implement.
type ServiceInterface interface {
	StartService()
	StopService()
}
```

### Creating a service.

To create a service, you need to have a struct which implements above interface function
`StartService`, `StopService`.

Since different services may have different ways to be created you may need to have a method
`NewServiceABC` for service ABC with its own hooked params which returns the service object.

### Action

Action is the input to operate Service Manager. We can send action to action channel of service
manager to start or stop a service.

```go
// Action is type of service action.
type Action struct {
	action      ActionType
	serviceType Type
	params      map[string]interface{}
}
```

### Resharding

Service Manager is very handy to transform a node role from validator to leader or anything
else. All we need to do is to stop all current services and start all services of the new role.

### LibP2P Integration

We have enabled libp2p based gossiping using pubsub. Nodes no longer send messages to individual
nodes. All message communication is via SendMessageToGroups function.

- There would be 4 topics for sending and receiving of messages

  - **GroupIDBeacon** This topic serves for consensus within the beaconchain
  - **GroupIDBeaconClient** This topic serves for receipt of staking transactions by beacon chain and broadcast of blocks (by beacon leader)
  - **GroupIDShard** (_under construction_) This topic serves for consensus related and pingpong messages within the shard
  - **GroupIDShardClient** (_under construction_) This topic serves to receive transactions from client and send confirmed blocks back to client. The shard leader (only) sends back the confirmed blocks.

- Beacon chain nodes need to subscribe to _TWO_ topics

  - **GroupIDBeacon**
  - **GroupIDBeaconClient**.

- Every new node other than beacon chain nodes
  - **GroupIDBeaconClient**
  - **GroupIDShard**
  - **GroupIDShardClient**
