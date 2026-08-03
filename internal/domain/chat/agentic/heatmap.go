package agentic

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// GitHub-style activity heatmap generation.
//
// Inspired by Claude Code's heatmap.ts — generates terminal-based
// contribution heatmaps from daily activity data. Uses percentile-based
// intensity levels (p25/p50/p75) with block characters.

// HeatmapIntensity represents the activity level for a cell.
type HeatmapIntensity int

const (
	IntensityNone   HeatmapIntensity = 0
	IntensityLow    HeatmapIntensity = 1
	IntensityMedium HeatmapIntensity = 2
	IntensityHigh   HeatmapIntensity = 3
	IntensityMax    HeatmapIntensity = 4
)

// HeatmapCell represents a single cell in the heatmap grid.
type HeatmapCell struct {
	Date      time.Time        `json:"date"`
	Count     int              `json:"count"`
	Intensity HeatmapIntensity `json:"intensity"`
}

// Percentiles holds the distribution breakpoints for intensity bucketing.
type Percentiles struct {
	P25 float64
	P50 float64
	P75 float64
}

// CalculatePercentiles computes p25/p50/p75 from daily activity counts.
// Only considers non-zero days.
func CalculatePercentiles(dailyActivity map[string]int) Percentiles {
	var nonZero []float64
	for _, count := range dailyActivity {
		if count > 0 {
			nonZero = append(nonZero, float64(count))
		}
	}

	if len(nonZero) == 0 {
		return Percentiles{}
	}

	sort.Float64s(nonZero)

	return Percentiles{
		P25: percentile(nonZero, 25),
		P50: percentile(nonZero, 50),
		P75: percentile(nonZero, 75),
	}
}

// percentile computes the pth percentile using linear interpolation.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	idx := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}

	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// GetIntensity maps a count to an intensity level using percentile buckets.
func GetIntensity(count int, pct Percentiles) HeatmapIntensity {
	if count == 0 {
		return IntensityNone
	}
	fc := float64(count)
	if fc <= pct.P25 {
		return IntensityLow
	}
	if fc <= pct.P50 {
		return IntensityMedium
	}
	if fc <= pct.P75 {
		return IntensityHigh
	}
	return IntensityMax
}

// HeatmapChar returns the block character for an intensity level.
func HeatmapChar(intensity HeatmapIntensity) string {
	switch intensity {
	case IntensityNone:
		return "░"
	case IntensityLow:
		return "▒"
	case IntensityMedium:
		return "▓"
	case IntensityHigh:
		return "█"
	case IntensityMax:
		return "█"
	default:
		return " "
	}
}

// HeatmapOptions configures heatmap generation.
type HeatmapOptions struct {
	// Weeks is the number of weeks to display. Default: 52.
	Weeks int
	// EndDate is the last date (inclusive). Default: today.
	EndDate time.Time
	// ShowMonthLabels adds month abbreviations above the grid. Default: true.
	ShowMonthLabels bool
	// ShowDayLabels adds day-of-week labels on the left. Default: true.
	ShowDayLabels bool
}

// GenerateHeatmap produces a 7-row × N-column grid of activity cells.
func GenerateHeatmap(dailyActivity map[string]int, opts HeatmapOptions) [][]HeatmapCell {
	if opts.Weeks <= 0 {
		opts.Weeks = 52
	}
	if opts.EndDate.IsZero() {
		opts.EndDate = time.Now()
	}

	pct := CalculatePercentiles(dailyActivity)

	// Find the starting Sunday.
	end := opts.EndDate
	// Walk back to the last Saturday to include a full week.
	for end.Weekday() != time.Saturday {
		end = end.AddDate(0, 0, 1)
	}
	start := end.AddDate(0, 0, -(opts.Weeks*7 - 1))
	// Walk back to Sunday.
	for start.Weekday() != time.Sunday {
		start = start.AddDate(0, 0, -1)
	}

	// Build grid: 7 rows (Sun-Sat) × N columns (weeks).
	totalDays := int(end.Sub(start).Hours()/24) + 1
	cols := (totalDays + 6) / 7

	grid := make([][]HeatmapCell, 7)
	for row := 0; row < 7; row++ {
		grid[row] = make([]HeatmapCell, cols)
	}

	current := start
	for col := 0; col < cols; col++ {
		for row := 0; row < 7; row++ {
			if current.After(opts.EndDate.AddDate(0, 0, 1)) {
				break
			}
			key := current.Format("2006-01-02")
			count := dailyActivity[key]
			grid[row][col] = HeatmapCell{
				Date:      current,
				Count:     count,
				Intensity: GetIntensity(count, pct),
			}
			current = current.AddDate(0, 0, 1)
		}
	}

	return grid
}

// RenderHeatmap produces a text representation of the heatmap.
func RenderHeatmap(dailyActivity map[string]int, opts HeatmapOptions) string {
	grid := GenerateHeatmap(dailyActivity, opts)
	if len(grid) == 0 || len(grid[0]) == 0 {
		return ""
	}

	var b strings.Builder
	dayLabels := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	// Month labels row.
	if opts.ShowMonthLabels && len(grid[0]) > 0 {
		if opts.ShowDayLabels {
			b.WriteString("    ") // indent for day labels
		}
		lastMonth := -1
		for col := 0; col < len(grid[0]); col++ {
			cell := grid[0][col]
			month := int(cell.Date.Month())
			if month != lastMonth && !cell.Date.IsZero() {
				b.WriteString(cell.Date.Format("Jan"))
				lastMonth = month
				col += 2 // skip next 2 cols for spacing
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}

	// Grid rows.
	for row := 0; row < 7; row++ {
		if opts.ShowDayLabels {
			fmt.Fprintf(&b, "%-4s", dayLabels[row])
		}
		for col := 0; col < len(grid[row]); col++ {
			b.WriteString(HeatmapChar(grid[row][col].Intensity))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// HeatmapSummary provides aggregate statistics for the heatmap period.
type HeatmapSummary struct {
	TotalActivity int     `json:"totalActivity"`
	ActiveDays    int     `json:"activeDays"`
	MaxDaily      int     `json:"maxDaily"`
	AvgDaily      float64 `json:"avgDaily"`
	Percentiles   Percentiles
}

// SummarizeActivity computes aggregate stats from daily activity.
func SummarizeActivity(dailyActivity map[string]int) HeatmapSummary {
	summary := HeatmapSummary{
		Percentiles: CalculatePercentiles(dailyActivity),
	}

	for _, count := range dailyActivity {
		summary.TotalActivity += count
		if count > 0 {
			summary.ActiveDays++
		}
		if count > summary.MaxDaily {
			summary.MaxDaily = count
		}
	}

	if summary.ActiveDays > 0 {
		summary.AvgDaily = float64(summary.TotalActivity) / float64(summary.ActiveDays)
	}

	return summary
}
