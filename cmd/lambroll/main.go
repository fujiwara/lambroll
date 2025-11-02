package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"

	"github.com/fujiwara/lambroll"
	"golang.org/x/sys/unix"
)

func main() {
	os.Exit(_main())
}

func _main() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, unix.SIGTERM)
	defer stop()

	exitCode, err := lambroll.CLI(ctx, lambroll.ParseCLI)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Warn("Interrupted")
		} else {
			slog.Error("FAILED", "error", err)
		}
	}
	return exitCode
}
