package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/ghalamif/AegisFlow"
)

func main() {
	flow, err := aegisflow.Conf("../../data/config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := flow.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("edge runtime exited: %v", err)
	}
}
