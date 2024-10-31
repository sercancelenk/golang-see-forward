package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sse-demo/forward"
	"time"
)

type Message struct {
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done(): // Client disconnected
			log.Println("Client disconnected")
			return
		case t := <-ticker.C:
			// Create a new JSON message
			message := Message{
				Timestamp: t.Format(time.RFC3339),
				Status:    "active",
			}
			jsonData, err := json.Marshal(message)
			if err != nil {
				log.Println("Failed to encode JSON:", err)
				continue
			}

			// Send JSON data as an SSE message
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}
}

func main() {

	forwarder := forward.NewSSEForwarder()

	go forwarder.ForwardSSEFromPrimary()

	http.HandleFunc("/primary-events", sseHandler)
	http.HandleFunc("/forwarded-events", forwarder.SseHandler)

	log.Println("Starting forwarding SSE server on :8097")
	log.Fatal(http.ListenAndServe(":8097", nil))
}
