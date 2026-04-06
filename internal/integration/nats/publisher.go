package natsintegration

import (
	"github.com/nats-io/nats.go"
)

// ConnPublisher is a thin adapter so we can pass NATS publishing into tools
// without pulling NATS into internal/types.
type ConnPublisher struct {
	conn *nats.Conn
}

func NewConnPublisher(conn *nats.Conn) *ConnPublisher {
	return &ConnPublisher{conn: conn}
}

func (p *ConnPublisher) Publish(subject string, payload []byte) error {
	if p == nil || p.conn == nil {
		return nil
	}
	// nats.Conn.Publish is async; errors are rare and surfaced as connection errors.
	p.conn.Publish(subject, payload)
	return nil
}
