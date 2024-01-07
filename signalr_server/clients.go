package signalr_server

import (
	"errors"
	"sync"
)

type Clients interface {
	All() Target
	Group(string) Target
	Connection(string) Target
	AddConnectionToGroup(connection string, group string) error
	RemoveConnectionFromGroup(connection string, group string) error
	addConnection(connectionCtx *connectionCtx)
	removeConnection(connection string)
	getConnection(connection string) *connectionCtx
}

type clientsImp struct {
	clientCtxMap *connectionSuite
	groupMap     *connectionGroup
}

func CreateDefaultClients() Clients {
	return clientsImp{
		clientCtxMap: &connectionSuite{},
		groupMap:     &connectionGroup{},
	}
}

type Target interface {
	Send(string, ...any)
}

func (cImp clientsImp) All() Target {
	return cImp.clientCtxMap
}

func (cImp clientsImp) Group(group string) Target {
	suite, ok := cImp.groupMap.Load(group)
	if !ok {
		return nil
	}
	return suite.(*connectionSuite)
}

func (cImp clientsImp) Connection(connection string) Target {
	ctx, ok := cImp.clientCtxMap.Load(connection)
	if !ok {
		return nil
	}
	return ctx.(*connectionCtx)
}

func (cImp clientsImp) AddConnectionToGroup(connection string, group string) error {
	ctx, ok := cImp.clientCtxMap.Load(connection)
	if !ok {
		return errors.New("connection not found")
	}
	clientCtxMap, _ := cImp.groupMap.LoadOrStore(group, &connectionSuite{})
	clientCtxMap.(*connectionSuite).Store(connection, ctx)
	return nil
}

func (cImp clientsImp) RemoveConnectionFromGroup(connection string, group string) error {
	_, ok := cImp.clientCtxMap.Load(connection)
	if !ok {
		return errors.New("connection not found")
	}
	suite, ok := cImp.groupMap.Load(group)
	if !ok {
		return errors.New("group not found")
	}
	suite.(*connectionSuite).Delete(connection)
	return nil
}

// RemoveConnection
func (cImp clientsImp) removeConnection(connection string) {
	cImp.clientCtxMap.Delete(connection)
	// remove connection from all groups
	cImp.groupMap.Range(func(key, value interface{}) bool {
		value.(*connectionSuite).Delete(connection)
		return true
	})
}

func (cImp clientsImp) addConnection(connectionCtx *connectionCtx) {
	cImp.clientCtxMap.Store(connectionCtx.connectionId, connectionCtx)
}

func (cImp clientsImp) getConnection(connectionId string) *connectionCtx {
	if value, ok := cImp.clientCtxMap.Load(connectionId); ok {
		return value.(*connectionCtx)
	} else {
		return nil
	}
}

type connectionSuite struct {
	sync.Map
}

type connectionGroup struct {
	sync.Map
}

func (suite *connectionSuite) All() Target {
	return suite
}

func (suite *connectionSuite) Send(method string, args ...any) {
	suite.Range(func(key, value interface{}) bool {
		client := value.(*connectionCtx)
		client.Send(method, args...)
		return true
	})
}
