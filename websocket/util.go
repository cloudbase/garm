package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	apiParams "github.com/cloudbase/garm/apiserver/params"
)

type MessageHandler func(msgType int, msg []byte) error

func NewReader(ctx context.Context, baseURL, pth, token string, handler MessageHandler) (*Reader, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	wsScheme := "ws"
	if parsedURL.Scheme == "https" {
		wsScheme = "wss"
	}
	u := url.URL{Scheme: wsScheme, Host: parsedURL.Host, Path: pth}
	header := http.Header{}
	header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	return &Reader{
		ctx:     ctx,
		url:     u,
		header:  header,
		handler: handler,
		done:    make(chan struct{}),
	}, nil
}

type Reader struct {
	ctx    context.Context
	url    url.URL
	header http.Header

	done    chan struct{}
	running bool

	handler MessageHandler

	conn     *websocket.Conn
	mux      sync.Mutex
	writeMux sync.Mutex
}

func (w *Reader) Stop() {
	w.mux.Lock()
	defer w.mux.Unlock()
	if !w.running {
		return
	}
	w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	w.conn.Close()
	close(w.done)
	w.running = false
}

func (w *Reader) Done() <-chan struct{} {
	return w.done
}

func (w *Reader) WriteMessage(messageType int, data []byte) error {
	// The websocket package does not support concurrent writes and panics if it
	// detects that one has occurred, so we need to lock the writeMux to prevent
	// concurrent writes to the same connection.
	w.writeMux.Lock()
	defer w.writeMux.Unlock()
	if !w.running {
		return fmt.Errorf("websocket is not running")
	}
	if err := w.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return w.conn.WriteMessage(messageType, data)
}

func (w *Reader) Start() error {
	w.mux.Lock()
	defer w.mux.Unlock()
	if w.running {
		return nil
	}

	c, response, err := websocket.DefaultDialer.Dial(w.url.String(), w.header)
	if err != nil {
		var resp apiParams.APIErrorResponse
		var msg string
		var status string
		if response != nil {
			if response.Body != nil {
				if err := json.NewDecoder(response.Body).Decode(&resp); err == nil {
					msg = resp.Details
				}
			}
			status = response.Status
		}
		return fmt.Errorf("failed to stream logs: %q %s (%s)", err, msg, status)
	}
	w.conn = c
	w.running = true
	go w.loop()
	go w.handlerReader()
	return nil
}

func (w *Reader) handlerReader() {
	defer w.Stop()
	w.writeMux.Lock()
	w.conn.SetReadLimit(maxMessageSize)
	w.conn.SetReadDeadline(time.Now().Add(pongWait))
	w.conn.SetPongHandler(func(string) error { w.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	w.writeMux.Unlock()
	for {
		msgType, message, err := w.conn.ReadMessage()
		if err != nil {
			if IsErrorOfInterest(err) {
				slog.With(slog.Any("error", err)).Error("reading log message")
			}
			return
		}
		if w.handler != nil {
			if err := w.handler(msgType, message); err != nil {
				slog.With(slog.Any("error", err)).Error("handling log message")
			}
		}
	}
}

func (w *Reader) loop() {
	defer w.Stop()
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.Done():
			return
		case <-ticker.C:
			w.writeMux.Lock()
			w.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := w.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				w.writeMux.Unlock()
				return
			}
			w.writeMux.Unlock()
		}
	}
}
