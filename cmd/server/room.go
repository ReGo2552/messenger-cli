package main

import (
	"encoding/json"
	"net"
	"strings"
	"sync"
	"time"

	"messenger-cli/internal/protocol"
)

const (
	queueSize    = 64
	writeTimeout = 5 * time.Second
)

type room struct {
	mu      sync.Mutex
	members map[string]*member
}

type member struct {
	conn net.Conn
	out  chan protocol.Packet
}

func (m *member) writeLoop() {
	defer m.conn.Close() // Wake the reader if a write fails.
	for p := range m.out {
		if err := m.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
			return
		}
		if err := protocol.WritePacket(m.conn, p); err != nil {
			return
		}
	}
}

func (r *room) handle(conn net.Conn) {
	client := &member{conn: conn, out: make(chan protocol.Packet, queueSize)}
	written := make(chan struct{})
	go func() {
		defer close(written)
		client.writeLoop()
	}()

	var nick string
	defer func() {
		r.mu.Lock()
		if r.members[nick] == client {
			delete(r.members, nick)
		}
		close(client.out)
		r.mu.Unlock()
		<-written
	}()

	reject := func(reason string) {
		payload, _ := json.Marshal(protocol.ErrorMessage{Message: reason})
		select {
		case client.out <- protocol.Packet{Type: protocol.TypeError, Payload: payload}:
		default:
			conn.Close()
		}
	}
	p, err := protocol.ReadPacket(conn)
	if err != nil {
		return
	}
	var hello protocol.Hello
	if p.Type != protocol.TypeHello || json.Unmarshal(p.Payload, &hello) != nil || strings.TrimSpace(hello.Nick) == "" {
		reject("Первым сообщением укажите ник")
		return
	}
	nick = strings.TrimSpace(hello.Nick)
	hello.Nick = nick
	payload, _ := json.Marshal(hello)
	r.mu.Lock()
	if _, taken := r.members[nick]; taken {
		r.mu.Unlock()
		reject("Ник уже занят")
		return
	}
	client.out <- protocol.Packet{Type: protocol.TypeHello, Payload: payload}
	r.members[nick] = client
	r.mu.Unlock()

	for {
		p, err := protocol.ReadPacket(conn)
		if err != nil {
			return
		}
		var msg protocol.Message
		if p.Type != protocol.TypeMessage || json.Unmarshal(p.Payload, &msg) != nil || strings.TrimSpace(msg.Text) == "" {
			reject("Ожидается непустое текстовое сообщение")
			return
		}
		msg.From = nick
		payload, err := json.Marshal(msg)
		if err != nil || len(payload) > int(protocol.MaxPayloadSize) {
			reject("Сообщение с ником превышает лимит кадра")
			return
		}
		r.broadcast(client, protocol.Packet{Type: protocol.TypeMessage, Payload: payload})
	}
}

func (r *room) broadcast(sender *member, packet protocol.Packet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for nick, recipient := range r.members {
		if recipient == sender {
			continue
		}
		select {
		case recipient.out <- packet:
		default:
			delete(r.members, nick)
			recipient.conn.Close()
		}
	}
}
