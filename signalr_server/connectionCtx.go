package signalr_server

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"reflect"
	"sync/atomic"
	"time"
)

type connectionCtx struct {
	conn         connection
	hub          hubInterface
	connectionId string
	lastMsg      atomic.Value
	eCh          chan error
	msgCh        chan []byte
	end          chan any
	prtl         protocol
}

func initConnectionCtx(connectionId string, conn connection, hub hubInterface) *connectionCtx {
	ctx := &connectionCtx{
		connectionId: connectionId,
		conn:         conn,
		hub:          hub,
		lastMsg:      atomic.Value{},
		eCh:          make(chan error),
		msgCh:        make(chan []byte),
		end:          make(chan any),
	}
	ctx.lastMsg.Store(time.Now())
	return ctx
}

func (ctx *connectionCtx) handshake() error {
	p, err := ctx.conn.read()
	if err != nil {
		return err
	}
	protocol := &jsonProtocol{}
	// todo: handle incomplete msg
	protocol.verifyAndRemoveMessageSeparator(p)
	handshakeRequest := &HandshakeRequest{}
	err = json.Unmarshal(p[:len(p)-1], handshakeRequest)
	if err != nil {
		return err
	}
	switch handshakeRequest.Protocol {
	case "json":
		ctx.prtl = &jsonProtocol{}
	case "messagepack":
		ctx.prtl = &msgpackProtocol{}
	default:
		return errors.New("unknown protocol:" + handshakeRequest.Protocol)
	}
	if ctx.connectionId == "" {
		ctx.connectionId = uuid.NewString()
	}

	handshakeResponse := &HandshakeResponse{}
	handshakeResponseBytes, err := json.Marshal(handshakeResponse)

	if err != nil {
		return err
	}
	err = ctx.conn.send(protocol.appendMessageSeparator(handshakeResponseBytes))
	if err != nil {
		return err
	}
	return nil
}

func (ctx *connectionCtx) handleInbound(hub hubInterface) {
	err := ctx.handshake()
	if err != nil {
		ctx.writeError(err)
		return
	}
	for {
		p, err := ctx.conn.read()
		if err != nil {
			ctx.writeError(err)
			return
		}

		p = ctx.prtl.verifyAndRemoveMessageSeparator(p)
		msg, err := ctx.prtl.unmarshal(p)
		if err != nil {
			ctx.writeError(err)
			return
		}
		switch m := msg.(type) {
		case PingMsg:
			LogDebug("ping")
			ctx.lastMsg.Store(time.Now())
		case Invocation:
			ctx.lastMsg.Store(time.Now())
			LogDebug(m)
			target := m.Target
			method := reflect.ValueOf(hub).MethodByName(target)

			if !method.IsValid() {
				ctx.writeError(err)
				return
			}
			values := make([]reflect.Value, len(m.Arguments))
			numIn := method.Type().NumIn()
			if numIn != len(m.Arguments) {
				ctx.writeError(errors.New("wrong number of arguments"))
				return
			}
			for i, argument := range m.Arguments {
				bytes := argument.([]byte)
				v, err := ctx.prtl.unmarshalArgument(bytes, method.Type().In(i))
				if err != nil {
					ctx.writeError(err)
					return
				}
				values[i] = v
			}
			// the method might take very long time. Use another goroutine
			go func() {
				response := method.Call(values)

				LogDebug(response)
				resultsArray := make([]any, len(response))
				for i, r := range response {
					resultsArray[i] = r.Interface()
				}
				var result any
				switch len(resultsArray) {
				case 0:
					result = nil
				case 1:
					result = resultsArray[0]
				default:
					result = resultsArray
				}
				if m.InvocationId != "" {
					completeMsg := Completion{
						Type:         CompletionType,
						InvocationId: m.InvocationId,
					}
					if m.Type == StreamInvocationType {
						if len(resultsArray) != 1 {
							ctx.writeError(errors.New("stream method should return one result"))
							return
						}

						v := reflect.ValueOf(result)
						if v.Kind() != reflect.Chan {
							ctx.writeError(errors.New("this is not a stream method"))
							return
						}
						for {
							receivedValue, ok := v.Recv()
							if !ok {
								break
							}
							invocationResult := StreamItem{
								Type:         StreamItemType,
								InvocationId: m.InvocationId,
								Item:         receivedValue.Interface(),
							}
							invocationResultBytes, err := ctx.prtl.marshal(invocationResult)
							if err != nil {
								completeMsg.Error = err.Error()
								break
							}
							ctx.writeMsg(invocationResultBytes)
						}
					} else {
						completeMsg.Result = result
					}
					invocationResultBytes, err := ctx.prtl.marshal(completeMsg)
					if err != nil {
						ctx.writeError(err)
						return
					}
					ctx.writeMsg(invocationResultBytes)
				} else {
					LogDebug("invocationId is empty")
				}
			}()
		default:
			ctx.writeError(errors.New("unknown message type"))
			return
		}
	}
}

func (ctx *connectionCtx) start() {
	ctx.hub.Clients().addConnection(ctx)

	go ctx.handleInbound(ctx.hub)
	// check ping
	go ctx.checkPingLoop(ctx.hub.GetOptions().PingTimeout)
	// send ping
	go ctx.writePingLoop(ctx.hub.GetOptions().PingInterval)
}

func (ctx *connectionCtx) checkPingLoop(timeout int) {
	ctx.lastMsg.Store(time.Now())
	for {
		time.Sleep(5 * time.Second)
		lastMsg := ctx.lastMsg.Load().(time.Time)
		if time.Now().Sub(lastMsg) > time.Duration(timeout)*time.Second {
			ctx.writeError(errors.New("ping timeout"))
			return
		}
	}
}

func (ctx *connectionCtx) writePingLoop(interval int) {
	select {
	case <-ctx.end:
		return
	case <-time.After(time.Duration(interval) * time.Second):
		ctx.writeMsg(ctx.prtl.pingMsg())
	}
}

func (ctx *connectionCtx) waitError() {
	err := <-ctx.eCh
	LogWarning("", err)
	close(ctx.end)
	ctx.closeGracefully()
}

func (ctx *connectionCtx) Send(method string, args ...any) {
	LogDebug("invoke client")
	invocation := Invocation{
		Type:      InvocationType,
		Target:    method,
		Arguments: args,
	}
	invocationBytes, err := ctx.prtl.marshal(invocation)
	if err != nil {
		ctx.writeError(err)
		return
	}
	ctx.writeMsg(invocationBytes)
}

func (ctx *connectionCtx) writeMsg(msg []byte) {
	err := ctx.conn.send(ctx.prtl.appendMessageSeparator(msg))
	if err != nil {
		ctx.writeError(err)
	}
}

func (ctx *connectionCtx) writeError(err error) {
	select {
	case <-ctx.end:
	case ctx.eCh <- err:
	}
}

func (ctx *connectionCtx) closeGracefully() {
	ctx.hub.Clients().removeConnection(ctx.connectionId)
}
