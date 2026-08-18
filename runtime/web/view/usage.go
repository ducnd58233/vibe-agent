package view

import (
	"strconv"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// UsageTotals aggregates token fields across events that reported usage.
type UsageTotals struct {
	Input     int
	Output    int
	CacheRead int
	Reported  bool
}

// SumUsage adds payload.usage across rows.
func SumUsage(rows []EventRow) UsageTotals {
	var totals UsageTotals
	for _, row := range rows {
		if !row.HasUsage || row.Usage == nil {
			continue
		}
		totals.Reported = true
		totals.Input += row.Usage.Input
		totals.Output += row.Usage.Output
		totals.CacheRead += row.Usage.CacheRead
	}
	return totals
}

// FormatToolbarTokens renders toolbar token text.
func FormatToolbarTokens(t UsageTotals) string {
	if !t.Reported {
		return "not reported"
	}
	return formatUsage(session.Usage{
		Input:     t.Input,
		Output:    t.Output,
		CacheRead: t.CacheRead,
	})
}

func formatUsage(u session.Usage) string {
	text := "in " + strconv.Itoa(u.Input) + " · out " + strconv.Itoa(u.Output)
	if u.CacheRead > 0 {
		text += " · cache read " + strconv.Itoa(u.CacheRead)
	}
	return text
}
