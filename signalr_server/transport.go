package signalr_server

import (
	"github.com/gorilla/websocket"
	"io"
	"net/http"
)

type connection interface {
	send([]byte) error
	read() ([]byte, error)
}

type webSocketConnection struct {
	ws          *websocket.Conn
	messageType int
}

func (c *webSocketConnection) send(msg []byte) error {
	if msg == nil {
		return nil
	}
	// todo: rethink this
	if msg[len(msg)-1] == recordSeparator {
		return c.ws.WriteMessage(websocket.TextMessage, msg)
	} else {
		return c.ws.WriteMessage(websocket.BinaryMessage, msg)
	}

}

func (c *webSocketConnection) read() ([]byte, error) {
	_, p, err := c.ws.ReadMessage()
	return p, err
}

type postDrivenConnection interface {
	connection
	readFromRequest(r *http.Request, end chan any) error
}

type postDrivenConnectionImp struct {
	fromHub chan []byte
	toHub   chan []byte
	end     chan any
}

func (c *postDrivenConnectionImp) send(msg []byte) error {
	select {
	case <-c.end:
		return io.EOF
	case c.fromHub <- msg:
		return nil
	}
}

func (c *postDrivenConnectionImp) read() ([]byte, error) {
	select {
	case <-c.end:
		return nil, io.EOF
	case msg := <-c.toHub:
		return msg, nil
	}
}

func (c *postDrivenConnectionImp) readFromRequest(r *http.Request, end chan any) error {
	p, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	select {
	case c.toHub <- p:
	case <-end:
	}
	return nil
}

type longPollingConnection struct {
	postDrivenConnectionImp
}

func (c *longPollingConnection) waitAndFlush(w http.ResponseWriter, end chan any) error {
	select {
	case p := <-c.fromHub:
		_, err := w.Write(p)
		return err
	case <-end:
	}
	return nil
}

type serverSentEventsConnection struct {
	postDrivenConnectionImp
}

func (c *serverSentEventsConnection) keepFlushing(flusher http.Flusher, w http.ResponseWriter, end chan any) error {
	for {
		select {
		case p := <-c.fromHub:
			res := make([]byte, 0, len(p)+10)
			res = append(res, []byte("data: ")...)
			res = append(res, p...)
			res = append(res, []byte("\r\n")...)
			res = append(res, []byte("\r\n")...)
			_, err := w.Write(res)
			if err != nil {
				return err
			}
			flusher.Flush()
		case <-end:
			return nil
		}
	}
}
