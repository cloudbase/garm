//go:build !windows
// +build !windows

package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"golang.org/x/term"
)

var sigs = make(chan os.Signal, 1)

func watchTermResize(ctx context.Context, resizeCh chan [2]int, sessionID uuid.UUID) {
	signal.Notify(sigs, syscall.SIGWINCH)

	for {
		select {
		case <-sigs:
			w, h, err := term.GetSize(int(os.Stdin.Fd()))
			if err == nil && sessionID != uuid.Nil {
				resizeCh <- [2]int{w, h}
			}
		case <-ctx.Done():
			return
		}
	}
}
