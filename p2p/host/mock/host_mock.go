// Code generated by MockGen. DO NOT EDIT.
// Source: host.go

// Package mock_p2p is a generated GoMock package.
package mock_p2p

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	p2p "github.com/harmony-one/harmony/p2p"
	go_libp2p_host "github.com/libp2p/go-libp2p-host"
	go_libp2p_peer "github.com/libp2p/go-libp2p-peer"
)

// MockHost is a mock of Host interface
type MockHost struct {
	ctrl     *gomock.Controller
	recorder *MockHostMockRecorder
}

// MockHostMockRecorder is the mock recorder for MockHost
type MockHostMockRecorder struct {
	mock *MockHost
}

// NewMockHost creates a new mock instance
func NewMockHost(ctrl *gomock.Controller) *MockHost {
	mock := &MockHost{ctrl: ctrl}
	mock.recorder = &MockHostMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockHost) EXPECT() *MockHostMockRecorder {
	return m.recorder
}

// GetSelfPeer mocks base method
func (m *MockHost) GetSelfPeer() p2p.Peer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSelfPeer")
	ret0, _ := ret[0].(p2p.Peer)
	return ret0
}

// GetSelfPeer indicates an expected call of GetSelfPeer
func (mr *MockHostMockRecorder) GetSelfPeer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSelfPeer", reflect.TypeOf((*MockHost)(nil).GetSelfPeer))
}

// Close mocks base method
func (m *MockHost) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockHostMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockHost)(nil).Close))
}

// AddPeer mocks base method
func (m *MockHost) AddPeer(arg0 *p2p.Peer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddPeer", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddPeer indicates an expected call of AddPeer
func (mr *MockHostMockRecorder) AddPeer(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddPeer", reflect.TypeOf((*MockHost)(nil).AddPeer), arg0)
}

// GetID mocks base method
func (m *MockHost) GetID() go_libp2p_peer.ID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetID")
	ret0, _ := ret[0].(go_libp2p_peer.ID)
	return ret0
}

// GetID indicates an expected call of GetID
func (mr *MockHostMockRecorder) GetID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetID", reflect.TypeOf((*MockHost)(nil).GetID))
}

// GetP2PHost mocks base method
func (m *MockHost) GetP2PHost() go_libp2p_host.Host {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetP2PHost")
	ret0, _ := ret[0].(go_libp2p_host.Host)
	return ret0
}

// GetPeerCount ...
func (m *MockHost) GetPeerCount() int {
	return 1 // TODO(ricl): port
}

// GetP2PHost indicates an expected call of GetP2PHost
func (mr *MockHostMockRecorder) GetP2PHost() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetP2PHost", reflect.TypeOf((*MockHost)(nil).GetP2PHost))
}

// ConnectHostPeer mocks base method
func (m *MockHost) ConnectHostPeer(arg0 p2p.Peer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ConnectHostPeer", arg0)
}

// ConnectHostPeer indicates an expected call of ConnectHostPeer
func (mr *MockHostMockRecorder) ConnectHostPeer(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConnectHostPeer", reflect.TypeOf((*MockHost)(nil).ConnectHostPeer), arg0)
}

// SendMessageToGroups mocks base method
func (m *MockHost) SendMessageToGroups(groups []p2p.GroupID, msg []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMessageToGroups", groups, msg)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMessageToGroups indicates an expected call of SendMessageToGroups
func (mr *MockHostMockRecorder) SendMessageToGroups(groups, msg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMessageToGroups", reflect.TypeOf((*MockHost)(nil).SendMessageToGroups), groups, msg)
}

// GroupReceiver mocks base method
func (m *MockHost) GroupReceiver(arg0 p2p.GroupID) (p2p.GroupReceiver, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GroupReceiver", arg0)
	ret0, _ := ret[0].(p2p.GroupReceiver)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GroupReceiver indicates an expected call of GroupReceiver
func (mr *MockHostMockRecorder) GroupReceiver(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GroupReceiver", reflect.TypeOf((*MockHost)(nil).GroupReceiver), arg0)
}
