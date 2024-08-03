package websocket

import (
	"errors"
	"net"

	"github.com/gorilla/websocket"
)

func IsErrorOfInterest(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, websocket.ErrCloseSent) {
		return false
	}

	if errors.Is(err, websocket.ErrBadHandshake) {
		return false
	}

	if errors.Is(err, net.ErrClosed) {
		return false
	}

	asCloseErr, ok := err.(*websocket.CloseError)
	if ok {
		switch asCloseErr.Code {
		case websocket.CloseNormalClosure, websocket.CloseGoingAway,
			websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure:
			return false
		}
	}

	return true
}
