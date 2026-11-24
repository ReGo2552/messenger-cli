package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"messenger-cli/internal/protocol"
)

func TestRoomDeliveryDisconnectAndShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- serve(ctx, listener) }()
	defer cancel()

	join := func(nick string) net.Conn {
		t.Helper()
		conn, err := net.DialTimeout("tcp", listener.Addr().String(), 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if err := protocol.WriteJSON(conn, protocol.TypeHello, protocol.Hello{Nick: nick}); err != nil {
			t.Fatal(err)
		}
		reply, err := protocol.ReadPacket(conn)
		if err != nil || reply.Type != protocol.TypeHello {
			t.Fatalf("join %s: reply %v, error %v", nick, reply, err)
		}
		return conn
	}
	alice := join("alice")
	bob := join("bob")
	leaver := join("leaver")
	leaver.Close()
	lines := []string{"Привет", "Ты здесь?"}
	for _, text := range lines {
		if err := protocol.WriteJSON(alice, protocol.TypeMessage, protocol.Message{Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	for _, text := range lines {
		p, err := protocol.ReadPacket(bob)
		if err != nil {
			t.Fatal(err)
		}
		var msg protocol.Message
		if err := json.Unmarshal(p.Payload, &msg); err != nil {
			t.Fatal(err)
		}
		if p.Type != protocol.TypeMessage || msg.From != "alice" || msg.Text != text {
			t.Fatalf("got type %d %+v, want message from alice with text %q", p.Type, msg, text)
		}
	}
	newcomer := join("newcomer")
	if err := protocol.WriteJSON(newcomer, protocol.TypeMessage, protocol.Message{Text: "Здесь"}); err != nil {
		t.Fatal(err)
	}
	reply, err := protocol.ReadPacket(alice)
	var msg protocol.Message
	if err != nil || reply.Type != protocol.TypeMessage || json.Unmarshal(reply.Payload, &msg) != nil || msg.From != "newcomer" || msg.Text != "Здесь" {
		t.Fatalf("after disconnect: reply %v, error %v", reply, err)
	}

	silent, err := net.DialTimeout("tcp", listener.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer silent.Close()
	join("last")
	cancel()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop with idle connections")
	}
	if err := silent.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := protocol.ReadPacket(silent); err != io.EOF {
		t.Fatalf("silent connection: got %v, want EOF", err)
	}
}

func TestSlowReaderIsDisconnected(t *testing.T) {
	r := &room{members: make(map[string]*member)}
	join := func(nick string) (net.Conn, <-chan struct{}) {
		t.Helper()
		server, client := net.Pipe()
		t.Cleanup(func() { client.Close() })
		if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			r.handle(server)
		}()
		if err := protocol.WriteJSON(client, protocol.TypeHello, protocol.Hello{Nick: nick}); err != nil {
			t.Fatal(err)
		}
		if p, err := protocol.ReadPacket(client); err != nil || p.Type != protocol.TypeHello {
			t.Fatalf("join %s: reply %v, error %v", nick, p, err)
		}
		return client, stopped
	}
	_, slowStopped := join("slow")
	fast, _ := join("fast")
	sender, _ := join("sender")

	sendAndRead := func(text string) {
		t.Helper()
		if err := protocol.WriteJSON(sender, protocol.TypeMessage, protocol.Message{Text: text}); err != nil {
			t.Fatal(err)
		}
		packet, err := protocol.ReadPacket(fast)
		var msg protocol.Message
		if err != nil || packet.Type != protocol.TypeMessage || json.Unmarshal(packet.Payload, &msg) != nil || msg.Text != text {
			t.Fatalf("fast reader: reply %v, error %v", packet, err)
		}
	}
	for i := 0; i < queueSize+2; i++ {
		sendAndRead("fill")
	}
	select {
	case <-slowStopped:
	case <-time.After(time.Second):
		t.Fatal("slow reader still connected")
	}
	join("slow")
	sendAndRead("after eviction")
}
