package cmd

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"
	"golang.org/x/term"
)

func watchTermResize(ctx context.Context, resizeCh chan [2]int, sessionID uuid.UUID) {
	var lastW, lastH int
	for {
		select {
		case <-ctx.Done():
			return
		default:
			w, h, err := term.GetSize(int(os.Stdin.Fd()))
			if err == nil && (w != lastW || h != lastH) && sessionID != uuid.Nil {
				lastW, lastH = w, h
				resizeCh <- [2]int{w, h}
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}
