package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"

	"github.com/fujiwara/lambroll"
	"golang.org/x/sys/unix"
)

// Version number
var Version = "current"

func main() {
	os.Exit(_main())
}

func _main() int {
	lambroll.Version = Version
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, unix.SIGTERM)
	defer stop()

	exitCode, err := lambroll.CLI(ctx, lambroll.ParseCLI)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Println("[warn] Interrupted")
		} else {
			log.Printf("[error] FAILED. %s", err)
		}
	}
	return exitCode
}
