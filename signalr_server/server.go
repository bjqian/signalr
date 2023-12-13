package signalr_server

import (
	"encoding/json"
	"github.com/google/uuid"
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
		hubVal := reflect.ValueOf(hub)
		if hubVal.Kind() == reflect.Ptr && !hubVal.IsNil() {
			hubName := reflect.Indirect(reflect.ValueOf(hub)).Type().Name()
			hubName = strings.ToLower(hubName)
			hub.init(CreateDefaultClients())
			hc := handlerContext{hub: hub}
			http.HandleFunc("/"+hubName+"/negotiate", hc.negotiate)
			http.HandleFunc("/"+hubName, hc.handler)
		} else {
			logFatal("hub is invalid", nil)
		}
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

func (hc handlerContext) negotiate(w http.ResponseWriter, r *http.Request) {
	// always return negotiation version 0
	negotiationResponse := &NegotiateResponse{
		ConnectionId:     uuid.NewString(),
		NegotiateVersion: 0,
		AvailableTransports: []TransportDescription{
			{
				Transport:       "WebSockets",
				TransferFormats: []string{"Text"},
			},
		},
	}
	responseBytes, err := json.Marshal(negotiationResponse)
	if err != nil {
		logError("", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBytes)
}

func (hc handlerContext) handler(w http.ResponseWriter, r *http.Request) {
	// skip negotiation
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logError("Upgrade error", err)
		return
	}
	defer conn.Close()

	connectionId := r.URL.Query().Get("id")
	// todo: validate connectionId

	// handshake
	ctx := connectionCtx{conn: conn,
		eCh:          make(chan error),
		msgCh:        make(chan []byte),
		closeCh:      make(chan any),
		connectionId: connectionId,
	}
	err = ctx.handshake()
	if err != nil {
		logError("", err)
		return
	}
	defer ctx.closeGracefully()

	hc.hub.Clients().addConnection(&ctx)
	defer hc.hub.Clients().removeConnection(ctx.connectionId)

	// the loop starts.

	go ctx.flushLoop()
	// handle inbound
	go ctx.handleInbound(hc.hub)
	// check ping
	go ctx.checkPingLoop(hc.hub.GetOptions().PingTimeout)
	// send ping
	go ctx.writePingLoop(hc.hub.GetOptions().PingInterval)

	err = <-ctx.eCh
	logWarning("", err)
	close(ctx.closeCh)
}
