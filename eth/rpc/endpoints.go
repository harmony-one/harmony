// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rpc

import (
	"net"
	"strings"

	"github.com/ethereum/go-ethereum/log"
)

// StartHTTPEndpoint starts the HTTP RPC endpoint, configured with cors/vhosts/modules
func StartHTTPEndpoint(endpoint string, apis []API, modules []string, rmf *RpcMethodFilter, cors []string, vhosts []string, timeouts HTTPTimeouts) (net.Listener, *Server, error) {
	// Generate the whitelist based on the allowed modules
	whitelist := make(map[string]bool)
	for _, module := range modules {
		whitelist[module] = true
	}
	// Register all the APIs exposed by the services
	handler := NewServer()
	registered := make(map[string]struct{})
	for _, api := range apis {
		if whitelist[api.Namespace] || (len(whitelist) == 0 && api.Public) {
			if err := handler.RegisterName(api.Namespace, api.Service, rmf); err != nil {
				log.Info("HTTP registration failed", "namespace", api.Namespace, "error", err)
				return nil, nil, err
			}
			registered[api.Namespace] = struct{}{}
			log.Debug("HTTP registered", "namespace", api.Namespace)
		}
	}
	logUnavailableModules("HTTP", modules, registered)
	// All APIs registered, start the HTTP listener
	var (
		listener net.Listener
		err      error
	)
	if listener, err = net.Listen("tcp", endpoint); err != nil {
		log.Warn("HTTP listener open failed", "endpoint", endpoint, "error", err)
		return nil, nil, err
	}
	go NewHTTPServer(cors, vhosts, timeouts, handler).Serve(listener)
	return listener, handler, err
}

// StartWSEndpoint starts a websocket endpoint
func StartWSEndpoint(endpoint string, apis []API, modules []string, rmf *RpcMethodFilter, wsOrigins []string, exposeAll bool) (net.Listener, *Server, error) {

	// Generate the whitelist based on the allowed modules
	whitelist := make(map[string]bool)
	for _, module := range modules {
		whitelist[module] = true
	}
	// Register all the APIs exposed by the services
	handler := NewServer()
	registered := make(map[string]struct{})
	for _, api := range apis {
		if exposeAll || whitelist[api.Namespace] || (len(whitelist) == 0 && api.Public) {
			if err := handler.RegisterName(api.Namespace, api.Service, rmf); err != nil {
				log.Info("WebSocket registration failed", "namespace", api.Namespace, "error", err)
				return nil, nil, err
			}
			registered[api.Namespace] = struct{}{}
			log.Debug("WebSocket registered", "service", api.Service, "namespace", api.Namespace)
		}
	}
	logUnavailableModules("WebSocket", modules, registered)
	// All APIs registered, start the HTTP listener
	var (
		listener net.Listener
		err      error
	)
	if listener, err = net.Listen("tcp", endpoint); err != nil {
		log.Warn("WebSocket listener open failed", "endpoint", endpoint, "error", err)
		return nil, nil, err
	}
	go NewWSServer(wsOrigins, handler).Serve(listener)
	return listener, handler, err

}

// StartIPCEndpoint starts an IPC endpoint.
func StartIPCEndpoint(ipcEndpoint string, apis []API, rmf *RpcMethodFilter) (net.Listener, *Server, error) {
	// Register all the APIs exposed by the services.
	handler := NewServer()
	registered := make([]string, 0, len(apis))
	regMap := make(map[string]struct{})
	for _, api := range apis {
		if err := handler.RegisterName(api.Namespace, api.Service, rmf); err != nil {
			log.Info("IPC registration failed", "namespace", api.Namespace, "error", err)
			return nil, nil, err
		}
		if _, ok := regMap[api.Namespace]; !ok {
			registered = append(registered, api.Namespace)
			regMap[api.Namespace] = struct{}{}
		}
		log.Debug("IPC registered", "namespace", api.Namespace)
	}
	log.Debug("IPCs registered", "namespaces", strings.Join(registered, ","))
	// All APIs registered, start the IPC listener.
	listener, err := ipcListen(ipcEndpoint)
	if err != nil {
		log.Warn("IPC listener open failed", "endpoint", ipcEndpoint, "error", err)
		return nil, nil, err
	}
	go handler.ServeListener(listener)
	return listener, handler, nil
}

func logUnavailableModules(transport string, modules []string, registered map[string]struct{}) {
	if len(modules) == 0 {
		return
	}
	unavailable := make([]string, 0)
	for _, module := range modules {
		if _, ok := registered[module]; !ok {
			unavailable = append(unavailable, module)
		}
	}
	if len(unavailable) == 0 {
		return
	}
	available := make([]string, 0, len(registered))
	for module := range registered {
		available = append(available, module)
	}
	log.Warn("Unavailable modules in "+transport+" API list", "unavailable", unavailable, "available", available)
}
