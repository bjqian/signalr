package signalr_server

type hubInterface interface {
	Clients() Clients
	GetOptions() Options
	init(clients Clients)
}

type Hub struct {
	clients Clients
	Options Options
}

func (hub *Hub) Clients() Clients {
	return hub.clients
}

func (hub *Hub) GetOptions() Options {
	return hub.Options
}

func (hub *Hub) init(clients Clients) {
	hub.clients = clients
}
