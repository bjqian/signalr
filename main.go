package main

import (
	"bjqian/signalr/signalr_server"
	_ "bjqian/signalr/signalr_server"
	"log"
	_ "log"
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

type A struct {
	B int
}

func main() {
	signalr_server.SetLogLevel(signalr_server.Debug)
	server := signalr_server.Server{}
	hub := &Chat{}
	hub.Options.PingInterval = 5000
	hub.Options.PingTimeout = 10000
	server.RegisterHubs(hub)
	server.Start()
}
