package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	listener, err := net.Listen("tcp", "127.0.0.1:9000")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s", listener.Addr())
	if err := serve(ctx, listener); err != nil {
		log.Print(err)
	}
}

func serve(ctx context.Context, listener net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	stopAccept := context.AfterFunc(ctx, func() { listener.Close() })
	r := &room{members: make(map[string]*member)}
	var sessions sync.WaitGroup
	defer func() {
		cancel()
		if stopAccept() {
			listener.Close()
		}
		sessions.Wait()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		sessions.Go(func() {
			stopClose := context.AfterFunc(ctx, func() { conn.Close() })
			defer stopClose()
			r.handle(conn)
		})
	}
}
