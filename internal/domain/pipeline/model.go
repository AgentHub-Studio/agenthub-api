package pipeline

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Pipeline represents a DAG-based execution pipeline belonging to an agent.
type Pipeline struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	AgentID     uuid.UUID `db:"agent_id"`
	Status      string    `db:"status"`
	Config      []byte    `db:"config"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Node is a single processing step within a pipeline.
type Node struct {
	ID         uuid.UUID `db:"id"`
	PipelineID uuid.UUID `db:"pipeline_id"`
	NodeType   string    `db:"node_type"`
	Name       string    `db:"name"`
	Config     []byte    `db:"config"`
	PositionX  float64   `db:"position_x"`
	PositionY  float64   `db:"position_y"`
	CreatedAt  time.Time `db:"created_at"`
}

// Edge represents a directed connection between two nodes in a pipeline.
type Edge struct {
	ID           uuid.UUID `db:"id"`
	PipelineID   uuid.UUID `db:"pipeline_id"`
	SourceNodeID uuid.UUID `db:"source_node_id"`
	TargetNodeID uuid.UUID `db:"target_node_id"`
	Label        string    `db:"label"`
	CreatedAt    time.Time `db:"created_at"`
}

// ErrCyclicGraph is returned when the pipeline graph contains a cycle.
var ErrCyclicGraph = errors.New("pipeline graph contains a cycle")

// ErrCyclicDependency is an alias for ErrCyclicGraph.
var ErrCyclicDependency = ErrCyclicGraph

// ErrDuplicateNodeName is returned when two nodes share the same name in a pipeline.
var ErrDuplicateNodeName = errors.New("pipeline has duplicate node names")
