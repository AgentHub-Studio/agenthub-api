package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- CalculatePercentiles ---

func TestCalculatePercentiles_Empty(t *testing.T) {
	pct := agentic.CalculatePercentiles(nil)
	assert.Equal(t, 0.0, pct.P25)
	assert.Equal(t, 0.0, pct.P50)
	assert.Equal(t, 0.0, pct.P75)
}

func TestCalculatePercentiles_AllZero(t *testing.T) {
	data := map[string]int{"2026-01-01": 0, "2026-01-02": 0}
	pct := agentic.CalculatePercentiles(data)
	assert.Equal(t, 0.0, pct.P25)
}

func TestCalculatePercentiles_SingleValue(t *testing.T) {
	data := map[string]int{"2026-01-01": 10}
	pct := agentic.CalculatePercentiles(data)
	assert.Equal(t, 10.0, pct.P25)
	assert.Equal(t, 10.0, pct.P50)
	assert.Equal(t, 10.0, pct.P75)
}

func TestCalculatePercentiles_Distribution(t *testing.T) {
	data := map[string]int{
		"2026-01-01": 1,
		"2026-01-02": 2,
		"2026-01-03": 3,
		"2026-01-04": 4,
		"2026-01-05": 5,
		"2026-01-06": 6,
		"2026-01-07": 7,
		"2026-01-08": 8,
		"2026-01-09": 9,
		"2026-01-10": 10,
	}
	pct := agentic.CalculatePercentiles(data)
	assert.Greater(t, pct.P25, 0.0)
	assert.Greater(t, pct.P50, pct.P25)
	assert.Greater(t, pct.P75, pct.P50)
}

// --- GetIntensity ---

func TestGetIntensity_Zero(t *testing.T) {
	pct := agentic.Percentiles{P25: 2, P50: 5, P75: 8}
	assert.Equal(t, agentic.IntensityNone, agentic.GetIntensity(0, pct))
}

func TestGetIntensity_Low(t *testing.T) {
	pct := agentic.Percentiles{P25: 5, P50: 10, P75: 15}
	assert.Equal(t, agentic.IntensityLow, agentic.GetIntensity(3, pct))
}

func TestGetIntensity_Medium(t *testing.T) {
	pct := agentic.Percentiles{P25: 5, P50: 10, P75: 15}
	assert.Equal(t, agentic.IntensityMedium, agentic.GetIntensity(7, pct))
}

func TestGetIntensity_High(t *testing.T) {
	pct := agentic.Percentiles{P25: 5, P50: 10, P75: 15}
	assert.Equal(t, agentic.IntensityHigh, agentic.GetIntensity(12, pct))
}

func TestGetIntensity_Max(t *testing.T) {
	pct := agentic.Percentiles{P25: 5, P50: 10, P75: 15}
	assert.Equal(t, agentic.IntensityMax, agentic.GetIntensity(20, pct))
}

// --- HeatmapChar ---

func TestHeatmapChar_AllLevels(t *testing.T) {
	assert.Equal(t, "░", agentic.HeatmapChar(agentic.IntensityNone))
	assert.Equal(t, "▒", agentic.HeatmapChar(agentic.IntensityLow))
	assert.Equal(t, "▓", agentic.HeatmapChar(agentic.IntensityMedium))
	assert.Equal(t, "█", agentic.HeatmapChar(agentic.IntensityHigh))
	assert.Equal(t, "█", agentic.HeatmapChar(agentic.IntensityMax))
}

// --- GenerateHeatmap ---

func TestGenerateHeatmap_Returns7Rows(t *testing.T) {
	data := map[string]int{"2026-03-15": 5}
	grid := agentic.GenerateHeatmap(data, agentic.HeatmapOptions{
		Weeks:   4,
		EndDate: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	})
	assert.Len(t, grid, 7)
}

func TestGenerateHeatmap_CorrectColumns(t *testing.T) {
	data := map[string]int{}
	grid := agentic.GenerateHeatmap(data, agentic.HeatmapOptions{
		Weeks:   4,
		EndDate: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	})
	require.Len(t, grid, 7)
	// Each row should have ~4 columns for 4 weeks.
	assert.GreaterOrEqual(t, len(grid[0]), 4)
}

func TestGenerateHeatmap_IncludesActivity(t *testing.T) {
	data := map[string]int{"2026-03-15": 10}
	grid := agentic.GenerateHeatmap(data, agentic.HeatmapOptions{
		Weeks:   4,
		EndDate: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	})

	found := false
	for _, row := range grid {
		for _, cell := range row {
			if cell.Count == 10 {
				found = true
				assert.NotEqual(t, agentic.IntensityNone, cell.Intensity)
			}
		}
	}
	assert.True(t, found, "should find cell with count=10")
}

func TestGenerateHeatmap_DefaultOptions(t *testing.T) {
	data := map[string]int{}
	grid := agentic.GenerateHeatmap(data, agentic.HeatmapOptions{})
	assert.Len(t, grid, 7)
	assert.GreaterOrEqual(t, len(grid[0]), 50, "default 52 weeks")
}

// --- RenderHeatmap ---

func TestRenderHeatmap_ProducesOutput(t *testing.T) {
	data := map[string]int{
		"2026-03-01": 3,
		"2026-03-05": 8,
		"2026-03-10": 15,
	}
	output := agentic.RenderHeatmap(data, agentic.HeatmapOptions{
		Weeks:           4,
		EndDate:         time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		ShowMonthLabels: true,
		ShowDayLabels:   true,
	})

	assert.NotEmpty(t, output)
	// Should contain block characters.
	assert.Contains(t, output, "░")
}

func TestRenderHeatmap_WithDayLabels(t *testing.T) {
	data := map[string]int{}
	output := agentic.RenderHeatmap(data, agentic.HeatmapOptions{
		Weeks:         4,
		EndDate:       time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		ShowDayLabels: true,
	})
	assert.Contains(t, output, "Mon")
	assert.Contains(t, output, "Fri")
}

// --- SummarizeActivity ---

func TestSummarizeActivity_Empty(t *testing.T) {
	summary := agentic.SummarizeActivity(nil)
	assert.Equal(t, 0, summary.TotalActivity)
	assert.Equal(t, 0, summary.ActiveDays)
	assert.Equal(t, 0, summary.MaxDaily)
}

func TestSummarizeActivity_Normal(t *testing.T) {
	data := map[string]int{
		"2026-01-01": 5,
		"2026-01-02": 0,
		"2026-01-03": 10,
		"2026-01-04": 3,
	}
	summary := agentic.SummarizeActivity(data)
	assert.Equal(t, 18, summary.TotalActivity)
	assert.Equal(t, 3, summary.ActiveDays)
	assert.Equal(t, 10, summary.MaxDaily)
	assert.InDelta(t, 6.0, summary.AvgDaily, 0.1)
}
