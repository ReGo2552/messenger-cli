package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	conn, err := net.Dial("tcp", "127.0.0.1:9000")
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}
	defer conn.Close()

	input := bufio.NewScanner(os.Stdin)
	if !input.Scan() {
		if err := input.Err(); err != nil {
			return fmt.Errorf("read input: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintln(conn, input.Text()); err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	reply := bufio.NewScanner(conn)
	if !reply.Scan() {
		if err := reply.Err(); err != nil {
			return fmt.Errorf("read reply: %w", err)
		}
		return fmt.Errorf("read reply: %w", io.ErrUnexpectedEOF)
	}
	if _, err := fmt.Fprintln(os.Stdout, reply.Text()); err != nil {
		return fmt.Errorf("print reply: %w", err)
	}
	return nil
}
