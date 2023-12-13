package signalr_server

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"reflect"
	"sync/atomic"
	"time"
)

type connectionCtx struct {
	conn         *websocket.Conn
	connectionId string
	lastMsg      atomic.Value
	eCh          chan error
	msgCh        chan []byte
	closeCh      chan any
}

func (ctx *connectionCtx) handshake() error {
	messageType, p, err := ctx.conn.ReadMessage()
	if err != nil {
		return err
	}

	if messageType == websocket.TextMessage {
		verifyRecordSeparator(p)
		handshakeRequest := &HandshakeRequest{}
		err = json.Unmarshal(p[:len(p)-1], handshakeRequest)
		if err != nil {
			return err
		}
	} else {
		logFatal("not text message", nil)
	}

	if ctx.connectionId == "" {
		ctx.connectionId = uuid.NewString()
	}

	handshakeResponse := &HandshakeResponse{}
	handshakeResponseBytes, err := json.Marshal(handshakeResponse)

	if err != nil {
		return err
	}
	err = ctx.conn.WriteMessage(websocket.TextMessage, appendRecordSeparator(handshakeResponseBytes))
	if err != nil {
		return err
	}
	return nil
}

func (ctx *connectionCtx) handleInbound(hub hubInterface) {
	for {
		messageType, p, err := ctx.conn.ReadMessage()
		if err != nil {
			ctx.writeError(err)
			return
		}
		if messageType == websocket.TextMessage {
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
					}
				}()
			default:
				ctx.writeError(errors.New("unknown message type"))
				return
			}
		} else {
			ctx.writeError(errors.New("not text message"))
			return
		}
	}
}

func (ctx *connectionCtx) flushLoop() {
	for {
		select {
		case <-ctx.closeCh:
			logDebug("connection ended")
			return
		case msg := <-ctx.msgCh:
			err := ctx.conn.WriteMessage(websocket.TextMessage, appendRecordSeparator(msg))
			if err != nil {
				ctx.writeError(err)
				return
			}
		}
	}
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
	case <-ctx.closeCh:
		return
	case <-time.After(time.Duration(interval) * time.Second):
		ctx.writeMsg(pingMsgBytes)
	}
}

func (ctx *connectionCtx) Send(method string, args ...any) {
	logDebug("invoke client")
	invocation := Invocation{
		Type:      InvocationType,
		Target:    method,
		Arguments: args,
	}
	invocationBytes, err := json.Marshal(invocation)
	// If should be a good place to handle error here
	if err != nil {
		ctx.writeError(err)
		return
	}
	ctx.writeMsg(invocationBytes)
}

func (ctx *connectionCtx) writeMsg(msg []byte) {
	select {
	case <-ctx.closeCh:
	case ctx.msgCh <- msg:
	}
}

func (ctx *connectionCtx) writeError(err error) {
	select {
	case <-ctx.closeCh:
	case ctx.eCh <- err:
	}
}

func (ctx *connectionCtx) closeGracefully() {

}
