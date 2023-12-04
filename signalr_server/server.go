package signalr_server

import (
	"github.com/gorilla/websocket"
	"net/http"
	"reflect"
	"strings"
)

type Server struct {
	hubs []hubInterface
}

type handlerContext struct {
	hub hubInterface
}

func (s *Server) RegisterHubs(hubs ...hubInterface) {
	s.hubs = append(s.hubs, hubs...)
}

func (s *Server) Start() {
	for _, hub := range s.hubs {
		hubName := reflect.TypeOf(hub).Name()
		hubName = strings.ToLower(hubName)
		hub.init(CreateDefaultClients())
		handler := handlerContext{hub: hub}.handler
		http.HandleFunc("/"+hubName, handler)
	}
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		logError("ListenAndServe: ", err)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (handleCtx handlerContext) handler(w http.ResponseWriter, r *http.Request) {
	// skip negotiation
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logError("Upgrade error", err)
		return
	}
	defer conn.Close()

	// handshake
	ctx := connectionCtx{conn: conn,
		eCh:     make(chan error),
		msgCh:   make(chan []byte),
		closeCh: make(chan any),
	}
	err = ctx.handshake()
	if err != nil {
		logError("", err)
		return
	}
	defer ctx.closeGracefully()

	handleCtx.hub.Clients().addConnection(&ctx)
	defer handleCtx.hub.Clients().removeConnection(ctx.connectionId)

	// the loop starts.

	go ctx.flushLoop()
	// handle inbound
	go ctx.handleInbound(handleCtx.hub)
	// check ping
	go ctx.checkPingLoop(handleCtx.hub.GetOptions().PingTimeout)
	// send ping
	go ctx.writePingLoop(handleCtx.hub.GetOptions().PingInterval)

	err = <-ctx.eCh
	logWarning("", err)
	close(ctx.closeCh)
}
