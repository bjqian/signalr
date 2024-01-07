package signalr_server

type NegotiateResponse struct {
	ConnectionId        string                 `json:"connectionId"`
	NegotiateVersion    int                    `json:"negotiateVersion"`
	AvailableTransports []TransportDescription `json:"availableTransports"`
}

type TransportDescription struct {
	Transport       string   `json:"transport"`
	TransferFormats []string `json:"transferFormats"`
}

type HandshakeRequest struct {
	Protocol string `json:"protocol"`
	Version  int    `json:"version"`
}

type HandshakeResponse struct {
	Error string `json:"error,omitempty"`
}

type BaseType struct {
	Type int `json:"type"`
}

type PingMsg struct {
	Type int `json:"type"`
}

type Invocation struct {
	Type         int    `json:"type"`
	InvocationId string `json:"invocationId"`
	Target       string `json:"target"`
	Arguments    []any  `json:"arguments"`
}

type Completion struct {
	Type         int    `json:"type"`
	InvocationId string `json:"invocationId"`
	Result       any    `json:"result"`
	Error        string `json:"error,omitempty"`
}

const (
	InvocationType = 1
	CompletionType = 3
	PingType       = 6
)

type transportProtocol int

const (
	WebSocket transportProtocol = iota
	ServerSentEvents
	LongPolling
)
