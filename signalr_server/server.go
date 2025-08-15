package signalr_server

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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

func (s *Server) BindHubs(handle func(pattern string, handler func(http.ResponseWriter, *http.Request))) {
	for _, hub := range s.hubs {
		hubVal := reflect.ValueOf(hub)
		if hubVal.Kind() == reflect.Ptr && !hubVal.IsNil() {
			hubName := reflect.Indirect(reflect.ValueOf(hub)).Type().Name()
			hubName = strings.ToLower(hubName)
			hub.init(CreateDefaultClients())
			hc := handlerContext{hub: hub}
			handle("/"+hubName+"/negotiate", hc.negotiate)
			handle("/"+hubName, hc.handler)
		} else {
			LogFatal("hub is invalid", nil)
		}
	}
}

func (s *Server) Start() {
	s.BindHubs(http.HandleFunc)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		LogError("ListenAndServe: ", err)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (hc handlerContext) negotiate(w http.ResponseWriter, r *http.Request) {
	// always return negotiation version 0
	negotiationResponse := &NegotiateResponse{
		ConnectionId:     uuid.NewString(),
		NegotiateVersion: 0,
		AvailableTransports: []TransportDescription{
			{
				Transport:       "WebSockets",
				TransferFormats: []string{"Text", "Binary"},
			},
			{
				Transport:       "ServerSentEvents",
				TransferFormats: []string{"Text"},
			},
			{
				Transport:       "LongPolling",
				TransferFormats: []string{"Text", "Binary"},
			},
		},
	}
	responseBytes, err := json.Marshal(negotiationResponse)
	if err != nil {
		LogError("", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBytes)
}

func (hc handlerContext) handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		connectionId := r.URL.Query().Get("id")
		if connectionId == "" {
			LogFatal("connectionId is empty", nil)
		}
		ctx := hc.hub.Clients().getConnection(connectionId)
		if ctx == nil {
			LogFatal("connection not found", nil)
		}
		con, ok := ctx.conn.(postDrivenConnection)
		if !ok {
			LogFatal("connection type error", nil)
		}
		err := con.readFromRequest(r, ctx.end)
		if err != nil {
			ctx.writeError(err)
		}
	case "GET":
		protocol := checkProtocol(r)
		switch protocol {
		case LongPolling:
			hc.handleLongPolling(w, r)
		case ServerSentEvents:
			hc.handleServerSentEvents(w, r)
		case WebSocket:
			hc.HandleWebSocket(w, r)
		default:
			LogFatal("protocol not supported", nil)
		}
	default:
		LogFatal("method not supported", nil)
	}
}

func (hc handlerContext) handleServerSentEvents(w http.ResponseWriter, r *http.Request) {
	connectionId := r.URL.Query().Get("id")
	if connectionId == "" {
		LogFatal("connectionId is empty", nil)
	}

	ctx := hc.hub.Clients().getConnection(connectionId)
	if ctx == nil {
		sseC := &serverSentEventsConnection{
			postDrivenConnectionImp: postDrivenConnectionImp{
				fromHub: make(chan []byte),
				toHub:   make(chan []byte),
			},
		}
		hub := shallowCopyHubInterface(hc.hub)
		hub.setContext(r.Context())
		ctx = initConnectionCtx(connectionId, sseC, hub)
		sseC.end = ctx.end
		ctx.start()
		go ctx.waitError()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		//w.Write([]byte("\r\n"))
		// Check if the ResponseWriter supports flushing
		flusher, ok := w.(http.Flusher)
		if !ok {
			LogFatal("Streaming unsupported!", nil)
		}
		flusher.Flush()
		err := sseC.keepFlushing(flusher, w, ctx.end)
		if err != nil {
			ctx.writeError(err)
		}
	} else {
		LogFatal("sse connection reconnect", nil)
	}
}

func (hc handlerContext) handleLongPolling(w http.ResponseWriter, r *http.Request) {
	connectionId := r.URL.Query().Get("id")
	if connectionId == "" {
		LogFatal("connectionId is empty", nil)
	}

	ctx := hc.hub.Clients().getConnection(connectionId)
	if ctx == nil {
		lpc := &longPollingConnection{
			postDrivenConnectionImp: postDrivenConnectionImp{
				fromHub: make(chan []byte),
				toHub:   make(chan []byte),
			},
		}
		hub := shallowCopyHubInterface(hc.hub)
		hub.setContext(r.Context())
		ctx = initConnectionCtx(connectionId, lpc, hub)
		lpc.end = ctx.end
		ctx.start()
		go ctx.waitError()
	} else {
		lpc, success := ctx.conn.(*longPollingConnection)
		if !success {
			LogFatal("connection type error", nil)
		}
		err := lpc.waitAndFlush(w, ctx.end)
		if err != nil {
			ctx.writeError(err)
			//w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
}

func (hc handlerContext) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		LogError("Upgrade error", err)
		return
	}
	defer conn.Close()

	connectionId := r.URL.Query().Get("id")
	wsc := &webSocketConnection{ws: conn}
	hub := shallowCopyHubInterface(hc.hub)
	hub.setContext(r.Context())
	ctx := initConnectionCtx(connectionId, wsc, hub)
	ctx.start()
	ctx.waitError()
}

func checkProtocol(r *http.Request) transportProtocol {
	connectionHeader := strings.ToLower(r.Header.Get("Connection"))
	upgradeHeader := strings.ToLower(r.Header.Get("Upgrade"))
	if strings.Contains(connectionHeader, "upgrade") && upgradeHeader == "websocket" {
		return WebSocket
	}
	acceptHeader := strings.ToLower(r.Header.Get("Accept"))
	if strings.Contains(acceptHeader, "text/event-stream") {
		return ServerSentEvents
	}
	return LongPolling
}
