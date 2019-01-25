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
	Start()
	Stop()
}

```

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
