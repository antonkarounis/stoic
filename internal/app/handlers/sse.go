package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/antonkarounis/stoic/internal/platform/web"
)

func SSE() http.HandlerFunc {
	return web.ConfigureSSE(func(ctx context.Context, messageChan chan string) {
		messageChan <- generateTime()

		go func() {
			for range time.NewTicker(time.Second).C {
				messageChan <- generateTime()
			}
		}()
	})
}

func generateTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
