package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"message": "Hello from EdgeFlow origin!", "path": "%s"}`, r.URL.Path)
	})
	fmt.Println("Origin server running on :9000")
	http.ListenAndServe(":9000", nil)
}
