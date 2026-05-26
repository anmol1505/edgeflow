package main

import (
	"fmt"
	"net/http"
	"strings"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Large enough response to trigger compression (>512 bytes)
		data := strings.Repeat(`{"message": "Hello from Origin 1!", "path": "` + r.URL.Path + `"},`, 20)
		fmt.Fprintf(w, `{"results": [%s]}`, data)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	fmt.Println("Origin 1 running on :9000")
	http.ListenAndServe(":9000", nil)
}
