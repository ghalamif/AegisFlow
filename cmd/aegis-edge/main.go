package main

import (
	"bufio"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ghalamif/AegisFlow"
)

//go:embed assets/banner_color.ansi
var bannerColor string 

//go:embed assets/banner_plain.txt
var bannerPlain string

func main() {
	fmt.Print(selectBanner())
	fmt.Println()
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	var err error

	switch cmd {
	case "run":
		err = runCommand(os.Args[2:])
	case "validate":
		err = validateCommand(os.Args[2:])
	case "stats":
		err = statsCommand(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		printUsage()
		err = fmt.Errorf("unknown command %q", cmd)
	}

	if err != nil {
		log.Fatalf("aegis-edge %s: %v", cmd, err)
	}
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("config", "./data/config.yaml", "Path to edge configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	flow, err := aegisflow.Conf(*cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return flow.Run(ctx)
}

func validateCommand(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	cfgPath := fs.String("config", "./data/config.yaml", "Path to configuration file to validate")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := aegisflow.LoadConfig(*cfgPath); err != nil {
		return err
	}
	fmt.Printf("config %s looks good ✅\n", *cfgPath)
	return nil
}

func selectBanner() string {
	if os.Getenv("NO_COLOR") != "" {
		return bannerPlain
	}
	return bannerColor
}

func statsCommand(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	url := fs.String("url", "http://localhost:9100/metrics", "Prometheus metrics endpoint")
	interval := fs.Duration("interval", 2*time.Second, "Refresh interval")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	fmt.Printf("Streaming metrics from %s (Ctrl+C to stop)\n", *url)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := printMetricsSnapshot(*url); err != nil {
				fmt.Fprintf(os.Stderr, "stats error: %v\n", err)
			}
		}
	}
}

func printMetricsSnapshot(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	targets := map[string]float64{
		"aegis_samples_ingested_total": 0,
		"aegis_queue_length":           0,
		"aegis_wal_size_bytes":         0,
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		for key := range targets {
			if strings.HasPrefix(line, key+" ") {
				var value float64
				if _, err := fmt.Sscanf(line, key+" %f", &value); err == nil {
					targets[key] = value
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	fmt.Printf("[%s] samples=%f queue=%f wal_bytes=%f\n",
		time.Now().Format(time.RFC3339),
		targets["aegis_samples_ingested_total"],
		targets["aegis_queue_length"],
		targets["aegis_wal_size_bytes"],
	)
	return nil
}

func printUsage() {
	fmt.Printf(`AegisFlow CLI

Usage:
  aegis-edge <command> [flags]

Commands:
  run        Start the edge runtime using the provided config (default)
  validate   Load and validate a config file without starting the runtime
  stats      Poll the Prometheus metrics endpoint and print live counters

Examples:
  aegis-edge run -config ./data/config.yaml
  aegis-edge validate -config ./data/config.yaml
  aegis-edge stats -url http://localhost:9100/metrics -interval 1s
`)
}
