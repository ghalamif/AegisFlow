package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/ghalamif/AegisFlow/pkg/aegisflow"
)

func main() {
	cfgPath := flag.String("config", "./data/config.yaml", "Path to edge configuration file")
	flag.Parse()

	cfg, err := aegisflow.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	rt, err := aegisflow.NewEdgeRuntime(cfg)
	if err != nil {
		log.Fatalf("runtime init: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := rt.Run(ctx); err != nil {
		log.Fatalf("runtime: %v", err)
	}
}
