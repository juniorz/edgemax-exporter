package api

import (
	"github.com/gorilla/websocket"
)

func topicsFor(topics []string) []topic {
	ts := make([]topic, 0, len(topics))
	for _, t := range topics {
		ts = append(ts, topic(t))
	}
	return ts
}

type Subscription struct {
	C   <-chan interface{}
	Err <-chan error

	done   chan interface{}
	stream *messageStream
}

func newSubscription(conn *websocket.Conn, sessionID string, topics ...string) (*Subscription, error) {
	req := subscriptionRequest{
		Subscribe: topicsFor(topics),
		SessionID: sessionID,
	}

	stream := newStream(conn)
	if err := stream.WriteJSON(req); err != nil {
		return nil, err
	}

	done := make(chan interface{})
	resC := make(chan interface{})
	errC := make(chan error, 1)

	go func() {
		defer close(resC)
		for {
			select {
			case <-done:
				return
			default:
				if err := stream.receiveNext(resC); err != nil {
					errC <- err
					return
				}
			}
		}
	}()

	return &Subscription{
		C:   resC,
		Err: errC,

		done:   done,
		stream: stream,
	}, nil
}

func (s *Subscription) Stop() {
	close(s.done)
}
