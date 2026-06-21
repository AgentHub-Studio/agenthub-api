package memory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldPruneStaleGeneral_RequiresAgeAndLowRelevance(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-24 * time.Hour)

	assert.False(t,
		shouldPruneStaleGeneral(now, now.Add(-2*time.Hour), cutoff, 0.9),
		"recent memories must not be pruned even when a high threshold is configured",
	)
	assert.False(t,
		shouldPruneStaleGeneral(now, now.Add(-48*time.Hour), cutoff, 0.1),
		"old memories above relevance threshold must be retained",
	)
	assert.True(t,
		shouldPruneStaleGeneral(now, now.Add(-48*time.Hour), cutoff, 0.9),
		"old memories below relevance threshold should be pruned",
	)
}
