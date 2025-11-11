package aegisflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ghalamif/AegisFlow/internal/adapters/observability"
	"github.com/ghalamif/AegisFlow/internal/adapters/opcua"
	"github.com/ghalamif/AegisFlow/internal/adapters/queue"
	"github.com/ghalamif/AegisFlow/internal/adapters/sink"
	"github.com/ghalamif/AegisFlow/internal/adapters/wal"
	"github.com/ghalamif/AegisFlow/internal/app/pipeline"
	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

// EdgeRuntime wires up the collector → WAL → queue → sink pipeline and exposes
// simple lifecycle hooks for embedding AegisFlow inside any Go service.
type EdgeRuntime struct {
	cfg          *Config
	policy       ports.Policy
	obs          ports.Observability
	wal          ports.WAL
	queue        ports.SampleQueue
	collector    ports.Collector
	transformer  ports.Transformer
	sink         ports.Sink
	db           *sql.DB
	metricsSrv   *http.Server
	gaugeStopCh  chan struct{}
	ingestDoneCh chan struct{}
}

// NewEdgeRuntime bootstraps the default adapters (OPC UA collector, file WAL,
// in-memory queue, Timescale sink, Prometheus observability).
func NewEdgeRuntime(cfg *Config) (*EdgeRuntime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	obs := observability.NewPromObs()

	walAdapter, err := wal.NewFileWAL(cfg.WAL.Dir)
	if err != nil {
		return nil, err
	}

	q := queue.NewMemQueue(cfg.Policy.MaxQueueLen)
	if err := replayWALIntoQueue(walAdapter, q, cfg.Policy, obs); err != nil {
		return nil, err
	}

	col, err := opcua.NewCollector(cfg.OPCUA)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", cfg.Timescale.ConnString)
	if err != nil {
		return nil, err
	}

	tsSink := sink.NewTimescaleSink(db, cfg.Timescale.Table)
	tr := &noopTransformer{}

	return &EdgeRuntime{
		cfg:         cfg,
		policy:      cfg.Policy,
		obs:         obs,
		wal:         walAdapter,
		queue:       q,
		collector:   col,
		transformer: tr,
		sink:        tsSink,
		db:          db,
	}, nil
}

// Start begins the edge + ingest pipelines and launches the observability stack.
// It returns immediately; call Run to block on a context instead.
func (e *EdgeRuntime) Start() error {
	if e == nil {
		return fmt.Errorf("edge runtime is nil")
	}
	if err := pipeline.RunEdgePipeline(e.collector, e.wal, e.queue, e.policy, e.obs); err != nil {
		return err
	}

	e.ingestDoneCh = make(chan struct{})
	go func() {
		pipeline.RunIngestPipeline(e.wal, e.queue, e.transformer, e.sink, e.policy, e.obs)
		close(e.ingestDoneCh)
	}()

	e.startMetrics()
	return nil
}

// Run starts the runtime and blocks until the provided context is cancelled.
// Upon cancellation it attempts a graceful shutdown.
func (e *EdgeRuntime) Run(ctx context.Context) error {
	if err := e.Start(); err != nil {
		return err
	}
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.Shutdown(shutdownCtx)
}

// Shutdown stops the collector, metrics server, and DB connection.
func (e *EdgeRuntime) Shutdown(ctx context.Context) error {
	var errs []error

	if e.gaugeStopCh != nil {
		close(e.gaugeStopCh)
	}

	if e.metricsSrv != nil {
		if err := e.metricsSrv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, err)
		}
	}

	if e.collector != nil {
		if err := e.collector.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	if e.db != nil {
		if err := e.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (e *EdgeRuntime) startMetrics() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	e.metricsSrv = &http.Server{
		Addr:    e.cfg.Metrics.Addr,
		Handler: mux,
	}

	go func() {
		if err := e.metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("metrics server exited: %v", err)
		}
	}()

	e.gaugeStopCh = make(chan struct{})
	go e.recordResourceGauges(e.gaugeStopCh, time.Second)
}

func (e *EdgeRuntime) recordResourceGauges(stop <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			stats := e.wal.Stats()
			e.obs.SetGauge("aegis_wal_size_bytes", float64(stats.SizeBytes))
			e.obs.SetGauge("aegis_queue_length", float64(e.queue.Len()))
		}
	}
}

func replayWALIntoQueue(walAdapter ports.WAL, q ports.SampleQueue, pol ports.Policy, obs ports.Observability) error {
	stats := walAdapter.Stats()
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
	err := walAdapter.Iterate(start, func(id ports.WALEntryID, sample *domain.Sample) error {
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

type noopTransformer struct{}

func (n *noopTransformer) Transform(s *domain.Sample) (*domain.Sample, error) { return s, nil }
func (n *noopTransformer) Version() uint16                                    { return 1 }
