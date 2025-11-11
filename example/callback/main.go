package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ghalamif/AegisFlow/pkg/aegisflow"
)

func main() {
	flow, err := aegisflow.Conf("../../data/config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callback := func(batch []aegisflow.Sample) error {
		for _, sample := range batch {
			fmt.Printf("%s sensor=%s seq=%d values=%v\n",
				sample.Timestamp.Format(time.RFC3339Nano),
				sample.SensorID,
				sample.Seq,
				sample.Values,
			)
		}
		return nil
	}

	if err := flow.Run(ctx, aegisflow.StreamOutCallback("stdout", callback)); err != nil && err != context.Canceled {
		log.Fatalf("runtime error: %v", err)
	}
}
