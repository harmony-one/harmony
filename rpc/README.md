Files that are mainly ported over from go-ethereum (might include tslint fix):
* rpc/client.go: the rpc client.
* rpc/endpoints.go: starting HTTP/WS/IPC endpoints.
* rpc/errors.go: error code handling.
* rpc/handler.go: handling requests, running methods.
* rpc/http.go: starting HTTP service.
* rpc/ipc_unix.go: util for ipc unix 
* rpc/ipc.go: util for general ipc
* rpc/json.go: json de-/serialization
* rpc/server.go: the rpc server
* rpc/stdio.go: stdio for ipc.
* rpc/subscription.go: ws subscriptions.
* rpc/types.go: type declarations
* rpc/websocket.go: ws connection.
* rpc/hmyapi/addrlock.go

Richard's changes:
* rpc/service.go: our service (http/ws/ipc) starter/stoper
* other files under `rpc/hmyapi/`.
* other files that are not in `rpc` folder. Added some util functions.