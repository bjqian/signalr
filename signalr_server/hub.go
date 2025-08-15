package signalr_server

import (
	"context"
	"reflect"
)

type hubInterface interface {
	Clients() Clients
	GetOptions() Options
	init(clients Clients)
	setCallerId(callerId string)
	setContext(c context.Context)
}

type Hub struct {
	clients Clients
	Options Options
	Caller  string
	Context context.Context
}

func (hub *Hub) Clients() Clients {
	return hub.clients
}

func (hub *Hub) setCallerId(caller string) {
	hub.Caller = caller
}

func (hub *Hub) setContext(c context.Context) {
	hub.Context = c
}

func (hub *Hub) GetOptions() Options {
	return hub.Options
}

func (hub *Hub) init(clients Clients) {
	hub.clients = clients
}

func shallowCopyHubInterface(h hubInterface) hubInterface {
	v := reflect.ValueOf(h)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		panic("expected pointer to struct")
	}
	copyVal := reflect.New(v.Elem().Type()).Elem()
	copyVal.Set(v.Elem())
	copyPtr := copyVal.Addr().Interface()
	return copyPtr.(hubInterface)
}
