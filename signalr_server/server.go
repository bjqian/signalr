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
			{
				Transport:       "LongPolling",
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
	protocol := checkProtocol(r)
	switch protocol {
	case LongPolling:
		hc.handleLongPolling(w, r)
	case WebSocket:
		hc.HandleWebSocket(w, r)
	default:
		logFatal("protocol not supported", nil)
	}
}

func (hc handlerContext) handleLongPolling(w http.ResponseWriter, r *http.Request) {
	connectionId := r.URL.Query().Get("id")
	if connectionId == "" {
		logFatal("connectionId is empty", nil)
	}

	ctx := hc.hub.Clients().getConnection(connectionId)
	switch r.Method {
	case "GET":
		if ctx == nil {
			lpc := &longPollingConnection{
				fromHub: make(chan []byte),
				toHub:   make(chan []byte),
			}
			ctx = initConnectionCtx(connectionId, lpc, hc.hub)
			lpc.end = ctx.end
			ctx.start()
			go ctx.waitError()
		} else {
			lpc, success := ctx.conn.(*longPollingConnection)
			if !success {
				logFatal("connection type error", nil)
			}
			err := lpc.waitAndFlush(w, ctx.end)
			if err != nil {
				ctx.writeError(err)
				//w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
	case "POST":
		if ctx == nil {
			logFatal("connection not found", nil)
		} else {
			lpc, success := ctx.conn.(*longPollingConnection)
			if !success {
				logFatal("connection type error", nil)
			}
			err := lpc.readFromRequest(r, ctx.end)
			if err != nil {
				ctx.writeError(err)
			}
		}
	default:
		logFatal("method not supported", nil)
	}
}

func (hc handlerContext) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logError("Upgrade error", err)
		return
	}
	defer conn.Close()

	connectionId := r.URL.Query().Get("id")
	wsc := &webSocketConnection{ws: conn}
	ctx := initConnectionCtx(connectionId, wsc, hc.hub)
	ctx.start()
	ctx.waitError()
}

func checkProtocol(r *http.Request) transportProtocol {
	connectionHeader := strings.ToLower(r.Header.Get("Connection"))
	upgradeHeader := strings.ToLower(r.Header.Get("Upgrade"))
	if strings.Contains(connectionHeader, "upgrade") && upgradeHeader == "websocket" {
		return WebSocket
	}

	return LongPolling
}
