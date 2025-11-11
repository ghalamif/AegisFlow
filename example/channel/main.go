package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ghalamif/AegisFlow"
)

func main() {
	flow, err := aegisflow.Conf("../../data/config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sink, batches, closeBatches := aegisflow.NewChannelSink("fanout", 32)
	defer closeBatches()

	go fanoutWorker("ingest", batches)

	if err := flow.Run(ctx, aegisflow.StreamOutSink(sink)); err != nil && err != context.Canceled {
		log.Fatalf("runtime error: %v", err)
	}
}

func fanoutWorker(name string, batches <-chan []aegisflow.Sample) {
	for batch := range batches {
		fmt.Printf("[%s] forwarding %d samples at %s\n", name, len(batch), time.Now().Format(time.RFC3339))
		// TODO: forward to downstream DB/API.
	}
}
