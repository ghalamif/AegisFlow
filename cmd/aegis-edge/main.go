package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"aegisflow/internal/adapters/observability"
	"aegisflow/internal/adapters/opcua"
	"aegisflow/internal/adapters/queue"
	"aegisflow/internal/adapters/sink"
	"aegisflow/internal/adapters/wal"
	"aegisflow/internal/app/config"
	"aegisflow/internal/app/pipeline"
	"aegisflow/internal/domain"
	"aegisflow/internal/ports"
)

func main() {
	cfgPath := flag.String("config", "./data/config.yaml", "Path to edge configuration file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	obs := observability.NewPromObs()

	w, err := wal.NewFileWAL(cfg.WAL.Dir)
	if err != nil {
		log.Fatalf("wal init: %v", err)
	}

	q := queue.NewMemQueue(cfg.Policy.MaxQueueLen)

	if err := replayWALIntoQueue(w, q, cfg.Policy, obs); err != nil {
		log.Fatalf("wal replay: %v", err)
	}

	col, err := opcua.NewCollector(cfg.OPCUA)
	if err != nil {
		log.Fatalf("opcua collector: %v", err)
	}

	if err := pipeline.RunEdgePipeline(col, w, q, cfg.Policy, obs); err != nil {
		log.Fatalf("edge pipeline: %v", err)
	}

	db, err := sql.Open("postgres", cfg.Timescale.ConnString)
	if err != nil {
		log.Fatalf("timescale connect: %v", err)
	}

	tsSink := sink.NewTimescaleSink(db, cfg.Timescale.Table)
	tr := &noopTransformer{}

	go serveMetrics(cfg.Metrics.Addr)
	go recordResourceGauges(obs, w, q, time.Second)
	go handleSignals(col, db)

	log.Printf("AegisFlow edge up. Metrics at http://%s/metrics", cfg.Metrics.Addr)

	pipeline.RunIngestPipeline(w, q, tr, tsSink, cfg.Policy, obs)
}

type noopTransformer struct{}

func (n *noopTransformer) Transform(s *domain.Sample) (*domain.Sample, error) { return s, nil }
func (n *noopTransformer) Version() uint16                                    { return 1 }

func serveMetrics(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("metrics server exited: %v", err)
	}
}

func recordResourceGauges(obs ports.Observability, w ports.WAL, q ports.SampleQueue, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		stats := w.Stats()
		obs.SetGauge("aegis_wal_size_bytes", float64(stats.SizeBytes))
		obs.SetGauge("aegis_queue_length", float64(q.Len()))
	}
}

func handleSignals(col ports.Collector, db *sql.DB) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("shutdown signal (%s) received; stopping collector and closing DB", sig)

	if err := col.Stop(); err != nil {
		log.Printf("collector stop error: %v", err)
	}

	if db != nil {
		_ = db.Close()
	}

	os.Exit(0)
}

func replayWALIntoQueue(wal ports.WAL, q ports.SampleQueue, pol ports.Policy, obs ports.Observability) error {
	stats := wal.Stats()
	if stats.LatestAppended == 0 {
		return nil
	}
	start := stats.OldestUncommitted
	if start == 0 || start > stats.LatestAppended {
		return nil
	}

	sleep := pol.IdleSleep
	if sleep <= 0 {
		sleep = 5 * time.Millisecond
	}

	var replayed int
	err := wal.Iterate(start, func(id ports.WALEntryID, sample *domain.Sample) error {
		for {
			if q.Enqueue(id, sample) {
				replayed++
				return nil
			}
			switch pol.OnQueueFull {
			case "drop", "reject":
				return fmt.Errorf("queue full during WAL replay")
			default:
				time.Sleep(sleep)
			}
		}
	})
	if err != nil {
		return err
	}
	if replayed > 0 {
		obs.LogInfo("wal_replay_complete",
			ports.Field{Key: "samples", Value: replayed},
			ports.Field{Key: "from_id", Value: start})
	}
	return nil
}
