// Package graph is a stub module for Graph RAG — see MA-13 in the Mastra
// adoption plan.
//
// The full implementation extracts entities/relations from documents during
// ingestion, persists them alongside pgvector embeddings, and enables hybrid
// retrieval (vector top-K + graph traversal). This file ships the minimal
// type vocabulary so subsequent PRs can land incrementally.
package graph

import "github.com/google/uuid"

// Entity is a node extracted from a document chunk.
type Entity struct {
	ID    uuid.UUID
	Type  string
	Name  string
	Props map[string]any
}

// Edge is a directed relation between two entities.
type Edge struct {
	ID       uuid.UUID
	FromID   uuid.UUID
	ToID     uuid.UUID
	Relation string
	Props    map[string]any
}

// Traversal describes a multi-hop graph query.
type Traversal struct {
	StartEntityID uuid.UUID
	MaxHops       int
	RelationTypes []string
}

// Result is a single path returned by the graph retriever.
type Result struct {
	Entities []Entity
	Edges    []Edge
}
