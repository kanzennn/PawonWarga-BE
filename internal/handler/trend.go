package handler

import (
	"fmt"
	"strconv"
	"time"

	"PawonWarga-BE/internal/repository"
)

// maxTrendPoints caps how many points a trend chart renders. bucketTrend
// widens the granularity (day -> week -> month -> quarter -> year) as the
// date range grows so a full year still reads as ~12 points instead of 365,
// and a multi-year range doesn't render an unreadable wall of daily ticks.
const maxTrendPoints = 31

// TrendBucket is one downsampled point on a sentiment trend chart — either a
// single day or an aggregated week/month/quarter/year, depending on
// bucketTrend's chosen granularity for the given range.
type TrendBucket struct {
	Label    string
	Positive int64
	Neutral  int64
	Negative int64
}

// bucketTrend downsamples daily sentiment rows (already ordered ascending by
// day, per CombinedTrend's SQL) into at most ~maxTrendPoints buckets. The
// granularity is picked from the total span so it scales with the date
// range: <=31 days stays daily, <=~7 months goes weekly, <=~2.6 years goes
// monthly, <=~7.8 years goes quarterly, beyond that yearly.
func bucketTrend(rows []repository.DailySentimentRow) []TrendBucket {
	if len(rows) == 0 {
		return []TrendBucket{}
	}

	spanDays := int(rows[len(rows)-1].Day.Sub(rows[0].Day).Hours()/24) + 1

	var keyFn func(time.Time) time.Time
	var labelFn func(start time.Time) string

	switch {
	case spanDays <= maxTrendPoints:
		keyFn = func(t time.Time) time.Time { return t }
		labelFn = formatDay
	case spanDays <= maxTrendPoints*7:
		keyFn = weekStart
		labelFn = func(t time.Time) string {
			end := t.AddDate(0, 0, 6)
			if t.Month() == end.Month() {
				return fmt.Sprintf("%d-%d %s", t.Day(), end.Day(), monthAbbr[t.Month()])
			}
			return fmt.Sprintf("%s - %s", formatDay(t), formatDay(end))
		}
	case spanDays <= maxTrendPoints*31:
		keyFn = func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		}
		labelFn = func(t time.Time) string { return fmt.Sprintf("%s %d", monthAbbr[t.Month()], t.Year()) }
	case spanDays <= maxTrendPoints*92:
		keyFn = func(t time.Time) time.Time {
			quarterMonth := time.Month((int(t.Month())-1)/3*3 + 1)
			return time.Date(t.Year(), quarterMonth, 1, 0, 0, 0, 0, t.Location())
		}
		labelFn = func(t time.Time) string { return fmt.Sprintf("Q%d %d", (int(t.Month())-1)/3+1, t.Year()) }
	default:
		keyFn = func(t time.Time) time.Time { return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location()) }
		labelFn = func(t time.Time) string { return strconv.Itoa(t.Year()) }
	}

	type bucketAcc struct {
		start                       time.Time
		positive, neutral, negative int64
	}

	order := make([]time.Time, 0)
	acc := make(map[time.Time]*bucketAcc)
	for _, row := range rows {
		key := keyFn(row.Day)
		b, ok := acc[key]
		if !ok {
			b = &bucketAcc{start: key}
			acc[key] = b
			order = append(order, key)
		}
		b.positive += row.Positive
		b.neutral += row.Neutral
		b.negative += row.Negative
	}

	buckets := make([]TrendBucket, len(order))
	for i, key := range order {
		b := acc[key]
		buckets[i] = TrendBucket{
			Label:    labelFn(b.start),
			Positive: b.positive,
			Neutral:  b.neutral,
			Negative: b.negative,
		}
	}
	return buckets
}

// weekStart returns the Monday of t's week.
func weekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}
