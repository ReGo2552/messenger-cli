package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"messenger-cli/internal/protocol"
)

func main() {
	nick := flag.String("nick", "", "ник в общей комнате")
	flag.Parse()
	if err := run(strings.TrimSpace(*nick)); err != nil {
		log.Fatal(err)
	}
}

func run(nick string) error {
	if nick == "" {
		return errors.New("укажите ник с помощью -nick")
	}
	conn, err := net.Dial("tcp", "127.0.0.1:9000")
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := protocol.WriteJSON(conn, protocol.TypeHello, protocol.Hello{Nick: nick}); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- receive(conn) }()
	go func() {
		input := bufio.NewScanner(os.Stdin)
		for input.Scan() {
			line := input.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			if err := protocol.WriteJSON(conn, protocol.TypeMessage, protocol.Message{Text: line}); err != nil {
				done <- err
				return
			}
		}
		if err := input.Err(); err != nil {
			done <- err
			return
		}
		// Let the server process all submitted lines before it closes its side.
		if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
			done <- err
		}
	}()
	return <-done
}

func receive(conn net.Conn) error {
	for {
		p, err := protocol.ReadPacket(conn)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch p.Type {
		case protocol.TypeHello:
			var hello protocol.Hello
			if err := json.Unmarshal(p.Payload, &hello); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Вы в общей комнате под ником %s\n", hello.Nick)
		case protocol.TypeMessage:
			var msg protocol.Message
			if err := json.Unmarshal(p.Payload, &msg); err != nil {
				return err
			}
			fmt.Printf("%s: %s\n", msg.From, msg.Text)
		case protocol.TypeError:
			var problem protocol.ErrorMessage
			if err := json.Unmarshal(p.Payload, &problem); err != nil {
				return err
			}
			return errors.New(problem.Message)
		default:
			return fmt.Errorf("неизвестный тип сообщения %d", p.Type)
		}
	}
}
