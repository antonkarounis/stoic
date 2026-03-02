package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
)

func Time() http.HandlerFunc {
	return framework.BuildSSEHandler(func(ctx context.Context, messageChan chan string) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		// Send initial time
		select {
		case messageChan <- generateTime():
		case <-ctx.Done():
			return
		}

		for {
			select {
			case <-ticker.C:
				select {
				case messageChan <- generateTime():
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	})
}

func generateTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
