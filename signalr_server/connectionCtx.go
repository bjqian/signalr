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

	verifyRecordSeparator(p)
	handshakeRequest := &HandshakeRequest{}
	err = json.Unmarshal(p[:len(p)-1], handshakeRequest)
	if err != nil {
		return err
	}

	if ctx.connectionId == "" {
		ctx.connectionId = uuid.NewString()
	}

	handshakeResponse := &HandshakeResponse{}
	handshakeResponseBytes, err := json.Marshal(handshakeResponse)

	if err != nil {
		return err
	}
	err = ctx.conn.send(appendRecordSeparator(handshakeResponseBytes))
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

		verifyRecordSeparator(p)
		baseType := &BaseType{}
		err = json.Unmarshal(p[:len(p)-1], baseType)
		if err != nil {
			ctx.writeError(err)
			return
		}
		switch baseType.Type {
		case PingType:
			logDebug("ping")
			ctx.lastMsg.Store(time.Now())
		case InvocationType:
			invocation := &Invocation{}
			err = json.Unmarshal(p[:len(p)-1], invocation)
			if err != nil {
				ctx.writeError(err)
				return
			}
			ctx.lastMsg.Store(time.Now())
			logDebug(invocation)
			target := invocation.Target
			method := reflect.ValueOf(hub).MethodByName(target)
			if !method.IsValid() {
				ctx.writeError(err)
				return
			}
			values := make([]reflect.Value, len(invocation.Arguments))
			for i, argument := range invocation.Arguments {
				values[i] = reflect.ValueOf(argument)
			}
			// the method might take very long time. Use another goroutine
			go func() {
				response := method.Call(values)
				logDebug(response)
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
				if invocation.InvocationId != "" {
					invocationResult := Completion{
						Type:         CompletionType,
						InvocationId: invocation.InvocationId,
						Result:       result,
					}
					invocationResultBytes, err := json.Marshal(invocationResult)
					if err != nil {
						ctx.writeError(err)
						return
					}
					ctx.writeMsg(invocationResultBytes)
				} else {
					logDebug("invocationId is empty")
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
		ctx.writeMsg(pingMsgBytes)
	}
}

func (ctx *connectionCtx) waitError() {
	err := <-ctx.eCh
	logWarning("", err)
	close(ctx.end)
	ctx.closeGracefully()
}

func (ctx *connectionCtx) Send(method string, args ...any) {
	logDebug("invoke client")
	invocation := Invocation{
		Type:      InvocationType,
		Target:    method,
		Arguments: args,
	}
	invocationBytes, err := json.Marshal(invocation)
	if err != nil {
		ctx.writeError(err)
		return
	}
	ctx.writeMsg(invocationBytes)
}

func (ctx *connectionCtx) writeMsg(msg []byte) {
	err := ctx.conn.send(appendRecordSeparator(msg))
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
