package main

import (
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	listenAddr := getenv("ROUTER_LISTEN_ADDR", ":5432")
	stateFile := getenv("ROUTER_STATE_FILE", "/ha-state/primary")
	defaultPort := getenv("ROUTER_DEFAULT_PORT", "55432")

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", listenAddr, err)
	}
	defer listener.Close()

	log.Printf("db router listening on %s", listenAddr)
	for {
		client, err := listener.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go handle(client, stateFile, defaultPort)
	}
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func handle(client net.Conn, stateFile, defaultPort string) {
	defer client.Close()

	target, err := currentTarget(stateFile, defaultPort)
	if err != nil {
		log.Printf("read target: %v", err)
		return
	}

	upstream, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		log.Printf("dial %s: %v", target, err)
		return
	}
	defer upstream.Close()

	errc := make(chan error, 2)
	go copyConn(errc, upstream, client)
	go copyConn(errc, client, upstream)
	<-errc
}

func currentTarget(stateFile, defaultPort string) (string, error) {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return "", err
	}

	target := strings.TrimSpace(string(data))
	if !strings.Contains(target, ":") {
		target += ":" + defaultPort
	}
	return target, nil
}

func copyConn(errc chan<- error, dst net.Conn, src net.Conn) {
	_, err := io.Copy(dst, src)
	errc <- err
}
