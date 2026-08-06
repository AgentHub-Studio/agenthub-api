package knowledgebase_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
)

func TestKnowledgeBaseRequestAliasesMustAgree(t *testing.T) {
	t.Run("create accepts either alias and equivalent values", func(t *testing.T) {
		for name, tc := range map[string]struct {
			body                            string
			rerankStrategy                  string
			rerankStrategySnake             string
			graphEnabled, graphEnabledSnake bool
		}{
			"camel": {
				body:           `{"rerankStrategy":"llm","graphEnabled":true}`,
				rerankStrategy: "llm",
				graphEnabled:   true,
			},
			"snake": {
				body:                `{"rerank_strategy":"llm","graph_enabled":true}`,
				rerankStrategySnake: "llm",
				graphEnabledSnake:   true,
			},
			"equivalent": {
				body:                `{"rerankStrategy":" llm ","rerank_strategy":"llm","graphEnabled":false,"graph_enabled":false}`,
				rerankStrategy:      " llm ",
				rerankStrategySnake: "llm",
			},
		} {
			t.Run(name, func(t *testing.T) {
				var req knowledgebase.CreateRequest
				require.NoError(t, json.Unmarshal([]byte(tc.body), &req))
				assert.Equal(t, tc.rerankStrategy, req.RerankStrategy)
				assert.Equal(t, tc.rerankStrategySnake, req.RerankStrategySnake)
				assert.Equal(t, tc.graphEnabled, req.GraphEnabled)
				assert.Equal(t, tc.graphEnabledSnake, req.GraphEnabledSnake)
			})
		}
	})

	t.Run("update accepts equivalent aliases", func(t *testing.T) {
		var req knowledgebase.UpdateRequest
		require.NoError(t, json.Unmarshal([]byte(`{"rerankStrategy":" llm ","rerank_strategy":"llm","graphEnabled":false,"graph_enabled":false}`), &req))
		require.NotNil(t, req.RerankStrategy)
		require.NotNil(t, req.RerankStrategySnake)
		require.NotNil(t, req.GraphEnabled)
		require.NotNil(t, req.GraphEnabledSnake)
		assert.Equal(t, " llm ", *req.RerankStrategy)
		assert.Equal(t, "llm", *req.RerankStrategySnake)
		assert.False(t, *req.GraphEnabled)
		assert.False(t, *req.GraphEnabledSnake)
	})

	t.Run("create rejects conflicting aliases", func(t *testing.T) {
		var req knowledgebase.CreateRequest
		err := json.Unmarshal([]byte(`{"rerankStrategy":"llm","rerank_strategy":"rrf","graphEnabled":false,"graph_enabled":true}`), &req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting")
	})

	t.Run("update rejects conflicting aliases", func(t *testing.T) {
		var req knowledgebase.UpdateRequest
		err := json.Unmarshal([]byte(`{"rerankStrategy":"llm","rerank_strategy":"rrf","graphEnabled":false,"graph_enabled":true}`), &req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting")
	})
}
