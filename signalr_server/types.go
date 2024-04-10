package signalr_server

import (
	"encoding/json"
)

const (
	InvocationType       = 1
	StreamItemType       = 2
	CompletionType       = 3
	StreamInvocationType = 4
	PingType             = 6
)

type transportProtocol int

const (
	WebSocket transportProtocol = iota
	ServerSentEvents
	LongPolling
)

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

type InvocationWithJsonRawArguments struct {
	Type         int               `json:"type"`
	InvocationId string            `json:"invocationId"`
	Target       string            `json:"target"`
	Arguments    []json.RawMessage `json:"arguments"`
}

type Completion struct {
	Type         int    `json:"type"`
	InvocationId string `json:"invocationId"`
	Result       any    `json:"result,omitempty"`
	Error        string `json:"error,omitempty"`
}

type StreamItem struct {
	Type         int    `json:"type"`
	InvocationId string `json:"invocationId"`
	Item         any    `json:"item"`
}
