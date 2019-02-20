## Node Architecture (WIP)

### Services

In Harmony network, a node can be treated as one of the roles: validator, leader, beacon validator, beacon leader depending on its context. With each role, a node can run a certian set of services.

For example, a leader needs to run explorer support service, client support service, syncing support service etc.. while a normal validator does not run such many.

### Service Manager

To support such behavior, we architecture Node logic with service manager which can wait for actions which each triggers its management operation such as starting some service, stopping some service.

Each service needs to implement minimal interace behavior like Start, Stop so that the service manager can handle those operation.

```
// ServiceInterface is the collection of functions any service needs to implement.
type ServiceInterface interface {
	StartService()
	StopService()
}

```

### Creating a service.

To create a service, you need to have an struct which implements above interface function `StartService`, `StopService`.

Since different services may have different ways to be created you may need to have a method `NewServiceABC` for service ABC with its own hooked params which returns the service object.

### Action

Action is the input to operate Service Manager. We can send action to action channel of service manager to start or stop a service.

```
// Action is type of service action.
type Action struct {
	action      ActionType
	serviceType Type
	params      map[string]interface{}
}
```

### Resharding

Service Manager is very handy to transform a node role from validator to leader or anything else. All we need to do is to stop all current services and start all services of the new role.

### LibP2P Integration

We have enabled libp2p based gossiping using pubsub. Nodes no longer send messages to individual nodes.
All message communication is via SendMessageToGroups function.

* Beacon chain nodes need to subscribe to TWO topics
** one is beacon chain topic itself: GroupIDBeacon
** another one is beacon client topic: GroupIDBeaconClient. Only Beacon Chain leader needs to send to this topic.

* Every new node other than beacon chain nodes needs to subscribe to THREE topic. This also include txgen program.
** one is beacon chain client topic => It is used to send staking transaction, and receive beacon chain blocks to determine the sharding info and randomness
** one is shard consensus itself => It is used for within shard consensus, pingpong messages
** one is client of the shard => It is used to receive tx from client, and send block back to client like txgen. Only shard Leader needs to send to this topic.
