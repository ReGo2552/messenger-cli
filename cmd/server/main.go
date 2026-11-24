package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	const addr = "127.0.0.1:9000"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()
	log.Printf("listening on %s", addr)

	conn, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accept connection: %w", err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read message: %w", err)
		}
		return nil
	}

	if _, err := fmt.Fprintln(conn, scanner.Text()); err != nil {
		return fmt.Errorf("send echo: %w", err)
	}
	return nil
}
