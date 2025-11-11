package opcua

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"aegisflow/internal/domain"
	"aegisflow/internal/ports"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// Config captures the runtime details required to open an OPC UA session.
type Config struct {
	Endpoint         string        `yaml:"endpoint"`
	Username         string        `yaml:"username"`
	Password         string        `yaml:"password"`
	SecurityMode     string        `yaml:"security_mode"`
	SecurityPolicy   string        `yaml:"security_policy"`
	ApplicationName  string        `yaml:"application_name"`
	PublishInterval  time.Duration `yaml:"publish_interval"`
	SamplingInterval time.Duration `yaml:"sampling_interval"`
	Nodes            []NodeConfig  `yaml:"nodes"`
}

// NodeConfig defines a monitored tag/node.
type NodeConfig struct {
	NodeID   string `yaml:"node_id"`
	SensorID string `yaml:"sensor_id"`
	ValueKey string `yaml:"value_key"`
}

func (c *Config) ApplyDefaults() {
	if c.SecurityMode == "" {
		c.SecurityMode = "None"
	}
	if c.SecurityPolicy == "" {
		c.SecurityPolicy = "None"
	}
	if c.ApplicationName == "" {
		c.ApplicationName = "AegisFlow Edge"
	}
	if c.PublishInterval <= 0 {
		c.PublishInterval = 250 * time.Millisecond
	}
	if c.SamplingInterval < 0 {
		c.SamplingInterval = 0
	}
	for i := range c.Nodes {
		if c.Nodes[i].SensorID == "" {
			c.Nodes[i].SensorID = c.Nodes[i].NodeID
		}
		if c.Nodes[i].ValueKey == "" {
			c.Nodes[i].ValueKey = "value"
		}
	}
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if len(c.Nodes) == 0 {
		return errors.New("at least one node must be configured")
	}
	return nil
}

type Collector struct {
	cfg       Config
	client    *opcua.Client
	sub       *opcua.Subscription
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	handleMap map[uint32]NodeConfig
	seq       map[string]uint64
	mu        sync.Mutex
	started   bool
}

func NewCollector(cfg Config) (*Collector, error) {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Collector{
		cfg: cfg,
		seq: make(map[string]uint64),
	}, nil
}

func (c *Collector) Start(out chan<- *domain.Sample) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("opcua collector already started")
	}
	c.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	clientOpts, err := c.buildClientOptions()
	if err != nil {
		cancel()
		return err
	}

	client, err := opcua.NewClient(c.cfg.Endpoint, clientOpts...)
	if err != nil {
		cancel()
		return fmt.Errorf("opcua new client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		cancel()
		return fmt.Errorf("opcua connect: %w", err)
	}

	notifyCh := make(chan *opcua.PublishNotificationData, len(c.cfg.Nodes)*4)
	sub, err := client.Subscribe(ctx, &opcua.SubscriptionParameters{
		Interval: c.cfg.PublishInterval,
	}, notifyCh)
	if err != nil {
		cancel()
		_ = client.Close(ctx)
		return fmt.Errorf("opcua subscribe: %w", err)
	}

	handleMap := make(map[uint32]NodeConfig, len(c.cfg.Nodes))
	for i, node := range c.cfg.Nodes {
		nodeID, err := ua.ParseNodeID(node.NodeID)
		if err != nil {
			c.cleanupOnError(ctx, cancel, sub, client)
			return fmt.Errorf("parse node id %q: %w", node.NodeID, err)
		}
		handle := uint32(i + 1)
		req := opcua.NewMonitoredItemCreateRequestWithDefaults(nodeID, ua.AttributeIDValue, handle)
		if c.cfg.SamplingInterval > 0 {
			req.RequestedParameters.SamplingInterval = float64(c.cfg.SamplingInterval / time.Millisecond)
		}
		res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, req)
		if err != nil {
			c.cleanupOnError(ctx, cancel, sub, client)
			return fmt.Errorf("monitor node %q: %w", node.NodeID, err)
		}
		if len(res.Results) == 0 {
			c.cleanupOnError(ctx, cancel, sub, client)
			return fmt.Errorf("monitor node %q failed: empty result", node.NodeID)
		}
		if res.Results[0].StatusCode != ua.StatusOK {
			c.cleanupOnError(ctx, cancel, sub, client)
			return fmt.Errorf("monitor node %q failed: %s", node.NodeID, res.Results[0].StatusCode)
		}
		handleMap[handle] = node
	}

	c.mu.Lock()
	c.client = client
	c.sub = sub
	c.cancel = cancel
	c.handleMap = handleMap
	c.started = true
	c.mu.Unlock()

	c.wg.Add(1)
	go c.consume(ctx, notifyCh, out)
	return nil
}

func (c *Collector) Stop() error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	cancel := c.cancel
	sub := c.sub
	client := c.client
	c.started = false
	c.cancel = nil
	c.sub = nil
	c.client = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	var err error
	if sub != nil {
		if e := sub.Cancel(ctx); e != nil && !errors.Is(e, context.Canceled) {
			err = errors.Join(err, e)
		}
	}
	if client != nil {
		if e := client.Close(ctx); e != nil && !errors.Is(e, context.Canceled) {
			err = errors.Join(err, e)
		}
	}

	c.wg.Wait()
	return err
}

func (c *Collector) consume(ctx context.Context, ch <-chan *opcua.PublishNotificationData, out chan<- *domain.Sample) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case notif := <-ch:
			if notif == nil {
				continue
			}
			if notif.Error != nil {
				log.Printf("opcua: notification error: %v", notif.Error)
				continue
			}
			c.processNotification(ctx, notif.Value, out)
		}
	}
}

func (c *Collector) processNotification(ctx context.Context, val interface{}, out chan<- *domain.Sample) {
	data, ok := val.(*ua.DataChangeNotification)
	if !ok {
		return
	}

	for _, item := range data.MonitoredItems {
		nodeCfg, ok := c.handleMap[item.ClientHandle]
		if !ok {
			continue
		}
		fv, ok := variantToFloat(item.Value.Value)
		if !ok {
			log.Printf("opcua: skipping node %s due to unsupported type %T", nodeCfg.NodeID, item.Value.Value)
			continue
		}

		ts := item.Value.ServerTimestamp
		if ts.IsZero() {
			ts = item.Value.SourceTimestamp
		}
		if ts.IsZero() {
			ts = time.Now()
		}

		seq := c.nextSeq(nodeCfg.SensorID)
		sample := &domain.Sample{
			SensorID:     nodeCfg.SensorID,
			Timestamp:    ts,
			Seq:          seq,
			Values:       map[string]float64{nodeCfg.ValueKey: fv},
			SourceNodeID: nodeCfg.NodeID,
		}

		select {
		case <-ctx.Done():
			return
		case out <- sample:
		}
	}
}

func (c *Collector) nextSeq(sensor string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := c.seq[sensor] + 1
	c.seq[sensor] = next
	return next
}

func (c *Collector) buildClientOptions() ([]opcua.Option, error) {
	opts := []opcua.Option{
		opcua.SecurityModeString(normalizeSecurityMode(c.cfg.SecurityMode)),
		opcua.SecurityPolicy(normalizeSecurityPolicy(c.cfg.SecurityPolicy)),
		opcua.ApplicationName(c.cfg.ApplicationName),
		opcua.AutoReconnect(true),
	}

	if c.cfg.Username != "" {
		opts = append(opts, opcua.AuthUsername(c.cfg.Username, c.cfg.Password))
	} else {
		opts = append(opts, opcua.AuthAnonymous())
	}

	return opts, nil
}

func (c *Collector) cleanupOnError(ctx context.Context, cancel context.CancelFunc, sub *opcua.Subscription, client *opcua.Client) {
	cancel()
	if sub != nil {
		_ = sub.Cancel(ctx)
	}
	if client != nil {
		_ = client.Close(ctx)
	}
}

func variantToFloat(v *ua.Variant) (float64, bool) {
	if v == nil {
		return 0, false
	}

	switch val := v.Value().(type) {
	case float32:
		return float64(val), true
	case float64:
		return val, true
	case int8:
		return float64(val), true
	case uint8:
		return float64(val), true
	case int16:
		return float64(val), true
	case uint16:
		return float64(val), true
	case int32:
		return float64(val), true
	case uint32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

func normalizeSecurityMode(mode string) string {
	switch strings.ToLower(mode) {
	case "sign":
		return "Sign"
	case "signandencrypt", "signencrypt", "sign_and_encrypt", "sign+encrypt":
		return "SignAndEncrypt"
	default:
		return "None"
	}
}

func normalizeSecurityPolicy(policy string) string {
	if policy == "" {
		return "None"
	}
	return policy
}

var _ ports.Collector = (*Collector)(nil)
