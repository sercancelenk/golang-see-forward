package forward

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

type SSEForwarder struct {
	clients []chan string
	mu      sync.Mutex
}

func NewSSEForwarder() *SSEForwarder {
	return &SSEForwarder{
		clients: make([]chan string, 0),
	}
}

func (f *SSEForwarder) addClient(ch chan string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients = append(f.clients, ch)
}

func (f *SSEForwarder) removeClient(ch chan string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, client := range f.clients {
		if client == ch {
			f.clients = append(f.clients[:i], f.clients[i+1:]...)
			break
		}
	}
}

func (f *SSEForwarder) ForwardSSEFromPrimary() {
	resp, err := http.Get("http://localhost:8097/primary-events") // Connects to the Primary SSE Server
	if err != nil {
		log.Fatalf("Failed to connect to primary SSE: %v", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(line[5:]) // Extract JSON data from the line
			f.broadcast(data)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading primary SSE stream: %v", err)
	}
}

func (f *SSEForwarder) broadcast(data string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, client := range f.clients {
		client <- data
	}
}

func (f *SSEForwarder) SseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	clientChan := make(chan string)
	f.addClient(clientChan)
	defer func() {
		f.removeClient(clientChan)
		close(clientChan)
	}()

	for {
		select {
		case <-r.Context().Done(): // Client disconnected
			log.Println("Client disconnected")
			return
		case data := <-clientChan: // Receive JSON data for this client
			_, err := fmt.Fprintf(w, "data: %s\n\n", data)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
