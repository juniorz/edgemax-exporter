package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/gorilla/websocket"
)

type messageStream struct {
	nextHeader int

	b *bytes.Buffer
	c *websocket.Conn
}

func newStream(conn *websocket.Conn) *messageStream {
	if conn == nil {
		return nil
	}

	return &messageStream{
		b: &bytes.Buffer{},
		c: conn,
	}
}

func (s *messageStream) ReadJSON(dst interface{}) error {
	m, err := s.Bytes()
	if err != nil {
		return err
	}

	// log.Printf("-> %s", string(m))

	return json.Unmarshal(m, dst)
}

func (s *messageStream) WriteJSON(src interface{}) error {
	body, err := json.Marshal(src)
	if err != nil {
		return err
	}

	w, err := s.c.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	// Header
	_, err = io.WriteString(w, strconv.Itoa(len(body))+"\n")
	if err != nil {
		return err
	}

	// Body
	_, err = w.Write(body)
	if err != nil {
		return err
	}

	return w.Close()
}

func (s *messageStream) Close() error {
	// Send close message
	err := s.c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return err
	}

	// TODO: Should it give the websocket a close deadline and wait?

	return s.c.Close()
}

func (s *messageStream) fillBuffer() error {
	t, r, err := s.c.NextReader()
	if err != nil {
		return err
	}

	if t != websocket.TextMessage {
		return fmt.Errorf("unerxpected message type: %d", t)
	}

	_, err = io.Copy(s.b, r)
	return err
}

func (s *messageStream) currentHeader() (int, error) {
	if s.nextHeader > 0 {
		return s.nextHeader, nil
	}

	// Read header
	header, err := s.b.ReadString('\n')
	if err == nil {
		s.nextHeader, err = strconv.Atoi(header[:len(header)-1])
		return s.nextHeader, err
	}

	// Put the data back
	s.b.WriteString(header)

	// Gets more data
	if err == io.EOF {
		err = s.fillBuffer() // Ignores io.EOF and refill buffer
	}

	if err != nil {
		return s.nextHeader, err
	}

	// Retry
	return s.currentHeader()
}

func (s *messageStream) Bytes() ([]byte, error) {
	n, err := s.currentHeader()
	if err != nil {
		return nil, err
	}

	// Has enough data
	if n <= s.b.Len() {
		s.nextHeader = 0
		return s.b.Next(n), nil
	}

	// Gets more data
	err = s.fillBuffer()
	if err != nil {
		return nil, err
	}

	// Retry
	return s.Bytes()
}

func (s *messageStream) receiveNext(C chan<- interface{}) error {
	resp := make(map[string]json.RawMessage)
	if err := s.ReadJSON(&resp); err != nil {
		return err
	}

	//TODO: Parse and dispatch
	for k, v := range resp {
		switch k {
		case "interfaces":
			r := interfaceStatResp{}
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}

			for _, stat := range r {
				C <- stat
			}
		case "system-stats":
			r := &SystemStat{}
			if err := json.Unmarshal(v, r); err != nil {
				return err
			}

			C <- r
		default:
			// log.Printf("-> %s", v)
			continue
			// case "export":
			// case "discover":
			// case "pon-stats":
			// case "num-routes":
			// case "config-change":
			// case "users":
		}
	}

	return nil
}
