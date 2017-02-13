package viewer

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

var (
	pingWait  = 70 * time.Second
	writeWait = 1 * time.Second
	// Maximum message size allowed from peer.
	maxMessageSize int64 = 512
)

type WsClient struct {
	out       io.Writer
	conn      *websocket.Conn
	interrupt chan struct{}
	done      chan struct{}
}

func WsConnect(addr string, out io.Writer, interrupt chan struct{}) {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/"}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("WsConnect: %v", err)
		return
	}
	client := &WsClient{
		out:       out,
		conn:      conn,
		done:      make(chan struct{}),
		interrupt: interrupt,
	}

	go client.pull()
	client.push()
}

func (c *WsClient) push() {
	defer func() {
		c.conn.Close()
	}()

	select {
	case <-c.done:
	case <-c.interrupt:
		log.Println("interrupted")
		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("write close:", err)
			return
		}
		select {
		case <-c.done:
		case <-time.After(time.Second):
		}
		return
	}
}

func (c *WsClient) pull() {
	defer func() {
		c.conn.Close()
		close(c.done)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPingHandler(func(message string) error {
		c.conn.SetReadDeadline(time.Now().Add(pingWait))
		err := c.conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(writeWait))
		if err == websocket.ErrCloseSent {
			return nil
		} else if e, ok := err.(net.Error); ok && e.Temporary() {
			return nil
		}
		return err
	})

	for {
		msgType, r, err := c.conn.NextReader()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			return
		}

		if msgType == websocket.TextMessage {
			msg, err := ioutil.ReadAll(r)
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
			c.out.Write([]byte("\r"))
			c.out.Write(msg)
			//if _, err := io.Copy(c.out, r); err != nil {
			//	log.Printf("error: %v", err)
			//	return
			//}
		}
		//if err := c.out.Close(); err != nil {
		//	log.Printf("error: %v", err)
		//	return
		//}
	}
}
