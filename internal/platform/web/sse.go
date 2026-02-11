package web

import (
	"context"
	"fmt"
	"net/http"
)

type SSEHandlerFunc func(context context.Context, messageChan chan string)

func ConfigureSSE(newClient SSEHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		rc := http.NewResponseController(w)
		done := r.Context().Done()
		clientChannel := make(chan string)

		go newClient(r.Context(), clientChannel)

		for {
			select {
			case data := <-clientChannel:
				_, err := fmt.Fprintf(w, "data: %s\n\n", data)
				if err != nil {
					return
				}
				rc.Flush()
			case <-done:
				return
			}
		}
	}
}
