package domain

import "time"

// Sample is the canonical unit of industrial telemetry in AegisFlow.
type Sample struct {
	SensorID     string             `json:"sensor_id"`
	Timestamp    time.Time          `json:"ts"`
	Seq          uint64             `json:"seq"`
	Values       map[string]float64 `json:"values"`
	SourceNodeID string             `json:"source_node_id"`
	TransformVer uint16             `json:"transform_ver"`
}
