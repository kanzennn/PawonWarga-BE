package handler

import (
	"strings"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

type KeywordHandler struct {
	keywordSvc service.KeywordService
}

func NewKeywordHandler(keywordSvc service.KeywordService) *KeywordHandler {
	return &KeywordHandler{keywordSvc: keywordSvc}
}

// KeywordItem's Growth is always "-" — see docs/api-contract.md's
// growth/rising/alerts. All three need a period-over-period comparison
// (this week vs last week, say), and the ingested data currently spans only
// ~4 days — "previous period" would be empty, making growth% meaningless
// (every keyword would read +100% or undefined). Revisit once enough
// history has accumulated to make a real window comparison.
type KeywordItem struct {
	Text      string `json:"text"`
	Value     int64  `json:"value"`
	Sentiment string `json:"sentiment"`
	Growth    string `json:"growth"`
	Mentions  string `json:"mentions"`
}

type KeywordStatsResponse struct {
	Tracked  int   `json:"tracked"`
	Rising   int   `json:"rising"`
	Negative int   `json:"negative"`
	Volume   int64 `json:"volume"`
}

// KeywordAlertResponse is always an empty list for now — alerts ("negative
// conversations up 24%") are a growth-derived feature, same blocker as
// KeywordItem.Growth above.
type KeywordAlertResponse struct {
	Title string `json:"title"`
	Desc  string `json:"desc"`
}

type KeywordListResponse struct {
	Items  []KeywordItem          `json:"items"`
	Stats  KeywordStatsResponse   `json:"stats"`
	Alerts []KeywordAlertResponse `json:"alerts"`
}

// dominantSentiment picks whichever sentiment has the most occurrences for
// a keyword, ties favoring positive then negative then neutral — same
// tie-break convention as computeDrivers (dashboard.go).
func dominantSentiment(positive, negative, neutral int64) model.Sentiment {
	switch {
	case positive >= negative && positive >= neutral:
		return model.SentimentPositive
	case negative >= positive && negative >= neutral:
		return model.SentimentNegative
	default:
		return model.SentimentNeutral
	}
}

type ListKeywordsQuery struct {
	Query     string `form:"q"`
	Sentiment string `form:"sentiment"`
	DateRangeQuery
}

// List godoc
// @Summary      List tracked keywords
// @Description  Top keywords extracted from posts+comments (see service.ExtractKeywordStats), with q/sentiment filtering the already-extracted list rather than re-running extraction. growth is always "-" and alerts is always empty — both need a period-over-period comparison the ingested data (currently ~4 days) can't yet support meaningfully. stats.* reflect the whole tracked set, not just the filtered items. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         keywords
// @Produce      json
// @Security     BearerAuth
// @Param        q          query     string  false  "Free-text filter on keyword text"
// @Param        sentiment  query     string  false  "positive, negative, neutral, or \"All Sentiments\""
// @Param        from       query     string  false  "Start date (YYYY-MM-DD, WIB) — omit for no lower bound"
// @Param        to         query     string  false  "End date (YYYY-MM-DD, WIB, inclusive) — omit for no upper bound"
// @Param        platform   query     string  false  "X, Instagram, TikTok, News, or YouTube — omit for all platforms"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Router       /keywords [get]
func (h *KeywordHandler) List(c *gin.Context) {
	lang := i18n.FromContext(c)

	var query ListKeywordsQuery
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

	overview, err := h.keywordSvc.GetKeywords(c.Request.Context(), from, to, string(platform))
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "keywords.list_failed"), err)
		return
	}

	var sentimentFilter model.Sentiment
	if query.Sentiment != "" && query.Sentiment != "All Sentiments" {
		sentimentFilter = model.Sentiment(strings.ToLower(query.Sentiment))
	}
	queryLower := strings.ToLower(query.Query)

	var negativeCount int
	var volume int64
	items := make([]KeywordItem, 0, len(overview.Keywords))

	for _, kw := range overview.Keywords {
		sentiment := dominantSentiment(kw.Positive, kw.Negative, kw.Neutral)
		volume += kw.Count
		if sentiment == model.SentimentNegative {
			negativeCount++
		}

		if queryLower != "" && !strings.Contains(strings.ToLower(kw.Text), queryLower) {
			continue
		}
		if sentimentFilter != "" && sentiment != sentimentFilter {
			continue
		}

		items = append(items, KeywordItem{
			Text:      kw.Text,
			Value:     kw.Count,
			Sentiment: string(sentiment),
			Growth:    "-",
			Mentions:  formatCompact(kw.Count),
		})
	}

	response.OK(c, i18n.T(lang, "keywords.list_ok"), KeywordListResponse{
		Items: items,
		Stats: KeywordStatsResponse{
			Tracked:  len(overview.Keywords),
			Rising:   0, // growth is deferred — see KeywordItem.Growth
			Negative: negativeCount,
			Volume:   volume,
		},
		Alerts: []KeywordAlertResponse{},
	})
}
