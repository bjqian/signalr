package main

import (
	"bjqian/signalr/signalr_server"
	"log"
)

type Chat struct {
	signalr_server.Hub
}

func (chat Chat) Do(message string) {
	log.Println(message)
	chat.Clients().All().Send("Receive", message+" from signalr_server")
}

func main() {
	signalr_server.SetLogLevel(signalr_server.Debug)
	server := signalr_server.Server{}
	hub := &Chat{}
	hub.Options.PingInterval = 5000
	hub.Options.PingTimeout = 120000
	server.RegisterHubs(hub)
	server.Start()
}
