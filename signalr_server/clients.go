package signalr_server

import "errors"

type Clients interface {
	All() Target
	Group(string) Target
	Connection(string) Target
	AddConnectionToGroup(connection string, group string) error
	RemoveConnectionFromGroup(connection string, group string) error
	addConnection(connectionCtx *connectionCtx)
	removeConnection(connection string)
}

type clientsImp struct {
	clientCtxMap connectionSuite
	groupMap     map[string]connectionSuite
}

func CreateDefaultClients() Clients {
	return clientsImp{
		clientCtxMap: make(connectionSuite),
		groupMap:     make(map[string]connectionSuite),
	}
}

type Target interface {
	Send(string, ...any)
}

func (cImp clientsImp) All() Target {
	return cImp.clientCtxMap
}

func (cImp clientsImp) Group(group string) Target {
	return cImp.groupMap[group]
}

func (cImp clientsImp) Connection(connection string) Target {
	return cImp.clientCtxMap[connection]
}

func (cImp clientsImp) AddConnectionToGroup(connection string, group string) error {
	connectionCtx, ok := cImp.clientCtxMap[connection]
	if !ok {
		return errors.New("connection not found")
	}
	suite, ok := cImp.groupMap[group]
	if !ok {
		suite = make(connectionSuite)
		cImp.groupMap[group] = suite
	}
	suite[connection] = connectionCtx
	return nil
}

func (cImp clientsImp) RemoveConnectionFromGroup(connection string, group string) error {
	_, ok := cImp.clientCtxMap[connection]
	if !ok {
		return errors.New("connection not found")
	}
	suite, ok := cImp.groupMap[group]
	if !ok {
		return errors.New("group not found")
	}
	delete(suite, connection)
	return nil
}

// RemoveConnection
// TODO: lock
// TODO: avoid iterating all groups
func (cImp clientsImp) removeConnection(connection string) {
	delete(cImp.clientCtxMap, connection)
	// remove connection from all groups
	for _, suite := range cImp.groupMap {
		delete(suite, connection)
	}
}

func (cImp clientsImp) addConnection(connectionCtx *connectionCtx) {
	cImp.clientCtxMap[connectionCtx.connectionId] = connectionCtx
}

type connectionSuite map[string]*connectionCtx

func (connections connectionSuite) Send(method string, args ...any) {
	for _, client := range connections {
		client.Send(method, args...)
	}
}
