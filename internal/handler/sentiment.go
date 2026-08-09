package handler

import (
	"fmt"
	"math"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

type SentimentHandler struct {
	sentimentSvc service.SentimentService
}

func NewSentimentHandler(sentimentSvc service.SentimentService) *SentimentHandler {
	return &SentimentHandler{sentimentSvc: sentimentSvc}
}

type SentimentSummary struct {
	Positive MetricValueResponse `json:"positive"`
	Negative MetricValueResponse `json:"negative"`
	Neutral  MetricValueResponse `json:"neutral"`
	Analyzed MetricValueResponse `json:"analyzed"`
}

type SentimentDistributionSlice struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Color string  `json:"color"`
}

type TrendInsightResponse struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Desc  string `json:"desc"`
	Tone  string `json:"tone"` // "positive" | "neutral" | "info" — a badge color, not a Sentiment
}

type CategorySentimentResponse struct {
	Category string `json:"category"`
	Positive int    `json:"positive"`
	Negative int    `json:"negative"`
	Neutral  int    `json:"neutral"`
}

type SentimentOverviewResponse struct {
	Score         int                          `json:"score"`
	ScoreMax      int                          `json:"score_max"`
	Description   string                       `json:"description"`
	Summary       SentimentSummary             `json:"summary"`
	Distribution  []SentimentDistributionSlice `json:"distribution"`
	Trend         []TrendPoint                 `json:"trend"`
	TrendInsights []TrendInsightResponse       `json:"trend_insights"`
	ByPlatform    []PlatformVolumeItem         `json:"by_platform"`
	ByCategory    []CategorySentimentResponse  `json:"by_category"`
	CategoryNote  string                       `json:"category_note"`
}

// sentimentScore is a simple 0-100 "health index": positive counts in full,
// neutral counts at half weight, negative doesn't count. Not a contract-
// specified formula (the frontend docs only show an example value) — this
// is a defensible default, easy to swap for something else later.
func sentimentScore(positive, neutral, total int64) int {
	if total == 0 {
		return 0
	}
	return int(math.Round((float64(positive) + 0.5*float64(neutral)) / float64(total) * 100))
}

func sentimentDescription(score int) string {
	switch {
	case score >= 70:
		return "Public sentiment remains healthy, with most conversations reflecting positive or neutral views on the MBG program."
	case score >= 40:
		return "Public sentiment is mixed, with a fairly even split of positive, neutral, and critical conversations about the MBG program."
	default:
		return "Public sentiment shows growing concern, with a significant share of conversations expressing negative views on the MBG program."
	}
}

func percentFloat1(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(part)/float64(total)*1000) / 10
}

// trendInsights derives three data points purely from the trend window —
// no separate taxonomy needed. tone is constrained to positive/neutral/info
// by the frontend's TrendInsight type (no "negative"/"danger" option here).
//
// buckets (raw counts) and trend (same buckets, converted to per-bucket
// percentages) must be the same length and index-aligned. Peak positive/
// negative compare percentages (share of that bucket's conversations) —
// correct to compare across buckets directly. "Most active" must compare
// buckets' counts instead: every bucket's percentages sum to ~100 by
// construction, so comparing trend's fields for volume would just pick
// whichever bucket rounds highest, not the one with the most conversations.
func trendInsights(buckets []TrendBucket, trend []TrendPoint) []TrendInsightResponse {
	if len(trend) == 0 {
		return []TrendInsightResponse{}
	}

	peakPositive, peakNegative := trend[0], trend[0]
	mostActiveIdx := 0
	mostActiveTotal := buckets[0].Positive + buckets[0].Neutral + buckets[0].Negative

	for i := 1; i < len(trend); i++ {
		if trend[i].Positive > peakPositive.Positive {
			peakPositive = trend[i]
		}
		if trend[i].Negative > peakNegative.Negative {
			peakNegative = trend[i]
		}
		total := buckets[i].Positive + buckets[i].Neutral + buckets[i].Negative
		if total > mostActiveTotal {
			mostActiveTotal = total
			mostActiveIdx = i
		}
	}

	return []TrendInsightResponse{
		{
			Label: "Peak positive",
			Value: fmt.Sprintf("%d%%", peakPositive.Positive),
			Desc:  fmt.Sprintf("Highest positive sentiment share, on %s", peakPositive.Day),
			Tone:  "positive",
		},
		{
			Label: "Peak negative",
			Value: fmt.Sprintf("%d%%", peakNegative.Negative),
			Desc:  fmt.Sprintf("Highest negative sentiment share, on %s", peakNegative.Day),
			Tone:  "info",
		},
		{
			Label: "Most active day",
			Value: trend[mostActiveIdx].Day,
			Desc:  "Highest conversation volume in the period",
			Tone:  "neutral",
		},
	}
}

func categoryNote(byCategory []CategorySentimentResponse) string {
	if len(byCategory) == 0 {
		return "Not enough classified conversations yet to highlight a category."
	}

	best := byCategory[0]
	for _, cat := range byCategory[1:] {
		if cat.Positive > best.Positive {
			best = cat
		}
	}
	return fmt.Sprintf("Positive sentiment is strongest in %s (%d%% positive).", best.Category, best.Positive)
}

// GetOverview godoc
// @Summary      Sentiment overview
// @Description  Aggregated sentiment stats backing the Sentiment Analysis page: score, summary metrics, distribution, trend, per-platform and per-category breakdowns. Combines posts AND comments (see repository.PostRepository's Combined* methods), like /dashboard/overview — unlike /mentions, which is post-only. by_category is keyword-matched (service.ClassifyTopicSentiment), not a stored classification. The platform filter applies to everything except by_platform, which always reports every platform (filtering a per-platform breakdown down to one platform would defeat its purpose). summary.*.change is always "-" — period-over-period comparison isn't implemented. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         sentiment
// @Produce      json
// @Security     BearerAuth
// @Param        from   query     string  false  "Start date (YYYY-MM-DD, WIB) — omit for no lower bound"
// @Param        to     query     string  false  "End date (YYYY-MM-DD, WIB, inclusive) — omit for no upper bound"
// @Param        platform  query  string  false  "X, Instagram, TikTok, News, or YouTube — omit for all platforms"
// @Success      200    {object}  response.Response
// @Failure      400    {object}  response.ErrorResponse
// @Router       /sentiment/overview [get]
func (h *SentimentHandler) GetOverview(c *gin.Context) {
	lang := i18n.FromContext(c)

	var query DateRangeQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.ValidationFailed(c, err)
		return
	}
	from, to, err := parseDateRange(query.From, query.To)
	if err != nil {
		response.BadRequest(c, i18n.T(lang, "validation.invalid_date"), err)
		return
	}
	platform := platformsByDisplayName[query.Platform] // "" for "All Platforms" / unknown / empty

	overview, err := h.sentimentSvc.GetOverview(c.Request.Context(), from, to, string(platform))
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "sentiment.overview.get_failed"), err)
		return
	}

	score := sentimentScore(overview.Positive, overview.Neutral, overview.Total)

	buckets := bucketTrend(overview.Trend)
	trend := make([]TrendPoint, len(buckets))
	for i, b := range buckets {
		bucketTotal := b.Positive + b.Neutral + b.Negative
		trend[i] = TrendPoint{
			Day:           b.Label,
			Positive:      percentOf(b.Positive, bucketTotal),
			Neutral:       percentOf(b.Neutral, bucketTotal),
			Negative:      percentOf(b.Negative, bucketTotal),
			PositiveCount: b.Positive,
			NeutralCount:  b.Neutral,
			NegativeCount: b.Negative,
		}
	}

	byCategory := make([]CategorySentimentResponse, len(overview.ByCategory))
	for i, cat := range overview.ByCategory {
		byCategory[i] = CategorySentimentResponse{
			Category: cat.Category.Name,
			Positive: percentOf(cat.Positive, cat.Total),
			Negative: percentOf(cat.Negative, cat.Total),
			Neutral:  percentOf(cat.Neutral, cat.Total),
		}
	}

	volumeByPlatform := make(map[model.Platform]struct {
		Value    int64
		Positive int64
		Negative int64
		Neutral  int64
		Total    int64
	}, len(overview.PlatformVolume))
	for _, row := range overview.PlatformVolume {
		volumeByPlatform[row.Platform] = struct {
			Value    int64
			Positive int64
			Negative int64
			Neutral  int64
			Total    int64
		}{row.Value, row.Positive, row.Negative, row.Neutral, row.Total}
	}

	byPlatform := make([]PlatformVolumeItem, 0, len(platformDisplayOrder))
	for _, name := range platformDisplayOrder {
		platformKey := platformsByDisplayName[name]
		row := volumeByPlatform[platformKey]
		byPlatform = append(byPlatform, PlatformVolumeItem{
			Name:         name,
			Value:        row.Value,
			Positive:     percentOf(row.Positive, row.Total),
			Negative:     percentOf(row.Negative, row.Total),
			Neutral:      percentOf(row.Neutral, row.Total),
			MentionCount: row.Total,
		})
	}

	totalStr := formatStatValue(overview.Total)

	response.OK(c, i18n.T(lang, "sentiment.overview.get_ok"), SentimentOverviewResponse{
		Score:       score,
		ScoreMax:    100,
		Description: sentimentDescription(score),
		Summary: SentimentSummary{
			Positive: MetricValueResponse{Value: formatPercent1(overview.Positive, overview.Total), Change: "-"},
			Negative: MetricValueResponse{Value: formatPercent1(overview.Negative, overview.Total), Change: "-"},
			Neutral:  MetricValueResponse{Value: formatPercent1(overview.Neutral, overview.Total), Change: "-"},
			Analyzed: MetricValueResponse{Value: totalStr, Change: "-"},
		},
		Distribution: []SentimentDistributionSlice{
			{Name: "Positive", Value: percentFloat1(overview.Positive, overview.Total), Color: "#67c74a"},
			{Name: "Negative", Value: percentFloat1(overview.Negative, overview.Total), Color: "#0c2033"},
			{Name: "Neutral", Value: percentFloat1(overview.Neutral, overview.Total), Color: "#f4b21b"},
		},
		Trend:         trend,
		TrendInsights: trendInsights(buckets, trend),
		ByPlatform:    byPlatform,
		ByCategory:    byCategory,
		CategoryNote:  categoryNote(byCategory),
	})
}
