package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func wsHandshake(w http.ResponseWriter, r *http.Request) (net.Conn, *bufio.ReadWriter, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	accept := computeAcceptKey(key)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijacking not supported")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}

	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"

	rw.WriteString(response)
	rw.Flush()

	return conn, rw, nil
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func readFrame(conn net.Conn) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := conn.Read(header); err != nil {
		return nil, err
	}

	payloadLen := int(header[1] & 0x7F)
	masked := header[1]&0x80 != 0

	if payloadLen == 126 {
		ext := make([]byte, 2)
		conn.Read(ext)
		payloadLen = int(ext[0])<<8 | int(ext[1])
	}

	var mask [4]byte
	if masked {
		conn.Read(mask[:])
	}

	payload := make([]byte, payloadLen)
	conn.Read(payload)

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return payload, nil
}

func writeFrame(conn net.Conn, msg []byte) error {
	frame := make([]byte, 2+len(msg))
	frame[0] = 0x81 // FIN + text frame
	frame[1] = byte(len(msg))
	copy(frame[2:], msg)
	_, err := conn.Write(frame)
	return err
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, _, err := wsHandshake(w, r)
	if err != nil {
		log.Println("handshake error:", err)
		return
	}
	defer conn.Close()

	fmt.Println("WebSocket client connected:", r.RemoteAddr)
	writeFrame(conn, []byte("Welcome to EdgeFlow WebSocket echo server!"))

	for {
		conn.SetDeadline(time.Now().Add(60 * time.Second))
		msg, err := readFrame(conn)
		if err != nil {
			break
		}
		response := fmt.Sprintf("[%s] Echo: %s", time.Now().Format("15:04:05"), string(msg))
		if err := writeFrame(conn, []byte(response)); err != nil {
			break
		}
		fmt.Printf("Echoed: %s\n", string(msg))
	}
	fmt.Println("WebSocket client disconnected:", r.RemoteAddr)
}

func main() {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
			http.Error(w, "not a websocket request", http.StatusBadRequest)
			return
		}
		wsHandler(w, r)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	fmt.Println("WebSocket server running on :9002")
	log.Fatal(http.ListenAndServe(":9002", nil))
}
