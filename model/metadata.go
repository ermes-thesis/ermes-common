package model

import (
	"time"
)

// Consistency defines the consistency level for data collections in the Ermes framework.
// It determines the guarantees provided when reading or writing data.
type Consistency uint8

const (
	// SequentialConsistent ensures sequential reads and writes.
	// Writes are visible immediately and consistently across all nodes,
	// but may require coordination with a leader.
	//
	// Use this Consistency if you prefer strong guarantees on the data of the
	// collection at the expenses of serverless functions performances
	SequentialConsistent Consistency = iota

	// EventualConsistent provides lower latency and higher availability by allowing
	// reads and writes to occur on any replica, with eventual convergence across replicas.
	//
	// Use this Consistency if you prefer higher serverless functions performances
	// at the expanses of the data consistency
	EventualConsistent
)

type Metadata struct {
	Name           string            `json:"name"`
	Owner          string            `json:"owner"`
	CreatedAt      time.Time         `json:"createdAt"`
	Groups         []string          `json:"groups"`
	Consistency    Consistency       `json:"consistency"`
	Geolocation    string            `json:"geolocation"`
	CustomMetadata map[string]string `json:"custom,omitempty"`
}
