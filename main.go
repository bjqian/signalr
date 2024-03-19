package main

import (
	"github.com/bjqian/signalr/signalr_server"
	_ "github.com/bjqian/signalr/signalr_server"
	"github.com/bjqian/signalr/signalr_service/rest_api"
	"log"
	_ "log"
	"os"
)

type Chat struct {
	signalr_server.Hub
}

type Foo struct {
	Bar string
}

func (chat Chat) Hi(foo Foo) bool {
	log.Println(foo.Bar)
	chat.Clients().All().Send("Receive", foo.Bar+" from signalr_server")
	return true
}

func (chat Chat) HiRaw(msg string) bool {
	chat.Clients().All().Send("Receive", msg)
	return true
}

func (chat Chat) HiArray(arr []int) bool {
	chat.Clients().All().Send("Receive", arr[1])
	return true
}

func main() {
	signalr_server.SetLogLevel(signalr_server.Debug)
	connectionString := os.Getenv("connectionString")
	client, err := rest_api.NewSignalRRestApiClient(connectionString, "chat")
	if err != nil {
		log.Fatal(err)
	}
	err = client.RemoveUserFromGroup("user", "group")
	err = client.BroadCastMessage("Receive", "golang")

	signalr_server.SetLogLevel(signalr_server.Debug)
	server := signalr_server.Server{}
	hub := &Chat{}
	hub.Options.PingInterval = 5000
	hub.Options.PingTimeout = 10000
	server.RegisterHubs(hub)
	server.Start()
}
