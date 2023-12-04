package signalr_server

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
	Type      int    `json:"type"`
	Target    string `json:"target"`
	Arguments []any  `json:"arguments"`
}

const (
	InvocationType = 1
	PingType       = 6
)
