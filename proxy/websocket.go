package proxy

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func ProxyWebSocket(w http.ResponseWriter, r *http.Request, targetHost string) {
	// Connect to origin via TCP
	targetConn, err := net.DialTimeout("tcp", targetHost, 10*time.Second)
	if err != nil {
		slog.Error("websocket: failed to connect to origin", "host", targetHost, "error", err)
		http.Error(w, "could not connect to origin", http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	// Hijack client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		slog.Error("websocket: hijacking not supported")
		http.Error(w, "websocket not supported", http.StatusInternalServerError)
		return
	}

	clientConn, bufrw, err := hijacker.Hijack()
	if err != nil {
		slog.Error("websocket: hijack failed", "error", err)
		return
	}
	defer clientConn.Close()

	// Write any buffered data first
	if bufrw.Reader.Buffered() > 0 {
		buffered := make([]byte, bufrw.Reader.Buffered())
		bufrw.Read(buffered)
		targetConn.Write(buffered)
	}

	// Forward original upgrade request to origin
	r.Write(targetConn)

	slog.Info("websocket proxying",
		"client", r.RemoteAddr,
		"origin", targetHost,
		"path", r.URL.Path,
	)

	// Bidirectional proxy
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(targetConn, clientConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientConn, targetConn)
		done <- struct{}{}
	}()
	<-done

	slog.Info("websocket closed", "client", r.RemoteAddr)
}
