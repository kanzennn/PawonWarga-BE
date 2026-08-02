package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	dashboardSvc service.DashboardService
}

func NewDashboardHandler(dashboardSvc service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardSvc: dashboardSvc}
}

// platformShortCode/platformSourceLabel are a second, shorter naming
// convention from platformDisplayNames (mention.go) — the frontend contract
// uses short badge codes ("X", "IG", "TT") for data_sources specifically,
// distinct from the full display names /mentions uses for platform/filters.
var platformShortCode = map[model.Platform]string{
	model.PlatformX:         "X",
	model.PlatformInstagram: "IG",
	model.PlatformTikTok:    "TT",
	model.PlatformYouTube:   "YT",
	model.PlatformNews:      "N",
}

var platformSourceLabel = map[model.Platform]string{
	model.PlatformX:         "X / Twitter",
	model.PlatformInstagram: "Instagram",
	model.PlatformTikTok:    "TikTok",
	model.PlatformYouTube:   "YouTube",
	model.PlatformNews:      "News",
}

// sentimentLabel is the capitalized display form of model.Sentiment —
// distinct from the lowercase wire value the frontend's Badge `tone` prop
// expects (see SentimentDriverResponse.Tone vs .Label).
var sentimentLabel = map[model.Sentiment]string{
	model.SentimentPositive: "Positive",
	model.SentimentNegative: "Negative",
	model.SentimentNeutral:  "Neutral",
}

var indoMonthAbbr = [...]string{
	"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
	"Jul", "Agu", "Sep", "Okt", "Nov", "Des",
}

type MetricValueResponse struct {
	Value  string `json:"value"`
	Change string `json:"change"`
}

type DashboardHeadline struct {
	PositiveShare      string `json:"positive_share"`
	TotalConversations string `json:"total_conversations"`
	Description        string `json:"description"`
}

type DashboardSummary struct {
	TotalMentions MetricValueResponse `json:"total_mentions"`
	Positive      MetricValueResponse `json:"positive"`
	Negative      MetricValueResponse `json:"negative"`
	Neutral       MetricValueResponse `json:"neutral"`
}

type TrendPoint struct {
	Day      string `json:"day"`
	Positive int    `json:"positive"`
	Neutral  int    `json:"neutral"`
	Negative int    `json:"negative"`
}

type TopicSlice struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
	Color string `json:"color"`
}

type WordCloudWordResponse struct {
	Text  string `json:"text"`
	Value int    `json:"value"`
}

// SentimentDriverResponse ranks a topic category by total engagement, with
// its prevailing sentiment — see service.computeDrivers. "Lainnya" never
// appears here (excluded as a non-theme catch-all). Value is the category's
// mention count (formatted), not engagement — engagement only drives the
// ranking order.
type SentimentDriverResponse struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Tone     string `json:"tone"`
	Label    string `json:"label"`
	Progress int    `json:"progress"`
}

type DataSourceStatus struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Active bool   `json:"active"`
}

type DashboardOverviewResponse struct {
	Headline          DashboardHeadline         `json:"headline"`
	Summary           DashboardSummary          `json:"summary"`
	Trend             []TrendPoint              `json:"trend"`
	TopicDistribution []TopicSlice              `json:"topic_distribution"`
	TopicTotal        string                    `json:"topic_total"`
	KeywordCloud      []WordCloudWordResponse   `json:"keyword_cloud"`
	RecentMentions    []MentionItem             `json:"recent_mentions"`
	Drivers           []SentimentDriverResponse `json:"drivers"`
	DataSources       []DataSourceStatus        `json:"data_sources"`
	LastUpdated       string                    `json:"last_updated"`
}

// formatGrouped renders a count with thousands separators ("128,430"),
// matching the frontend's headline/summary display convention (distinct from
// formatCompact's "128.4K" used in /mentions engagement figures).
func formatGrouped(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}

	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}

	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// formatPercent1 renders a one-decimal percentage ("72.4%"), matching the
// frontend's summary card convention.
func formatPercent1(part, total int64) string {
	if total == 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", float64(part)/float64(total)*100)
}

func formatIndoDay(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), indoMonthAbbr[t.Month()])
}

// relativeTimeID renders a coarse Indonesian relative-time string, matching
// the frontend contract's own example ("15 menit yang lalu").
func relativeTimeID(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "baru saja"
	case d < time.Hour:
		return fmt.Sprintf("%d menit yang lalu", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d jam yang lalu", int(d.Hours()))
	default:
		return fmt.Sprintf("%d hari yang lalu", int(d.Hours()/24))
	}
}

// GetOverview godoc
// @Summary      Dashboard overview
// @Description  Aggregated stats backing the main dashboard: headline, summary metrics, sentiment trend, per-platform volume, top keywords, topic distribution, and top sentiment drivers all combine posts AND comments (see repository.PostRepository's Combined* methods) — unlike /mentions, which is post-only. recent_mentions is the one exception and stays post-only (a bare comment has no url/location/category of its own). topic_distribution/keyword_cloud/drivers are keyword-matched, not a stored classification — see service.ClassifyTopics/computeDrivers. summary.*.change is always "-" — period-over-period comparison isn't implemented. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         dashboard
// @Produce      json
// @Security     BearerAuth
// @Param        from   query     string  false  "Start date (YYYY-MM-DD, WIB) — omit for no lower bound"
// @Param        to     query     string  false  "End date (YYYY-MM-DD, WIB, inclusive) — omit for no upper bound"
// @Success      200    {object}  response.Response
// @Failure      400    {object}  response.ErrorResponse
// @Router       /dashboard/overview [get]
func (h *DashboardHandler) GetOverview(c *gin.Context) {
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

	overview, err := h.dashboardSvc.GetOverview(c.Request.Context(), from, to)
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "dashboard.overview.get_failed"), err)
		return
	}

	totalStr := formatGrouped(overview.Total)

	buckets := bucketTrend(overview.Trend)
	trend := make([]TrendPoint, len(buckets))
	for i, b := range buckets {
		bucketTotal := b.Positive + b.Neutral + b.Negative
		trend[i] = TrendPoint{
			Day:      b.Label,
			Positive: percentOf(b.Positive, bucketTotal),
			Neutral:  percentOf(b.Neutral, bucketTotal),
			Negative: percentOf(b.Negative, bucketTotal),
		}
	}

	recentMentions := make([]MentionItem, len(overview.RecentPosts))
	for i, post := range overview.RecentPosts {
		recentMentions[i] = formatMentionItem(post)
	}

	keywordCloud := make([]WordCloudWordResponse, len(overview.Keywords))
	for i, kw := range overview.Keywords {
		keywordCloud[i] = WordCloudWordResponse{Text: kw.Text, Value: kw.Count}
	}

	topicDistribution := make([]TopicSlice, len(overview.Topics))
	for i, topic := range overview.Topics {
		topicDistribution[i] = TopicSlice{
			Name:  topic.Category.Name,
			Value: percentOf(int64(topic.Count), overview.Total),
			Color: topic.Category.Color,
		}
	}

	var maxDriverCount int64
	for _, d := range overview.Drivers {
		if d.Count > maxDriverCount {
			maxDriverCount = d.Count
		}
	}
	drivers := make([]SentimentDriverResponse, len(overview.Drivers))
	for i, d := range overview.Drivers {
		drivers[i] = SentimentDriverResponse{
			Name:  d.Category.Name,
			Value: formatCompact(d.Count),
			Tone:  string(d.Tone),
			Label: sentimentLabel[d.Tone],
			// Relative to the top driver, so the #1 theme always reads 100%
			// and the rest scale down from there — a leaderboard bar, not
			// a share of some larger total.
			Progress: percentOf(d.Count, maxDriverCount),
		}
	}

	volumeByPlatform := make(map[model.Platform]repository.PlatformVolumeRow, len(overview.PlatformVolume))
	for _, row := range overview.PlatformVolume {
		volumeByPlatform[row.Platform] = row
	}

	dataSources := make([]DataSourceStatus, 0, len(platformDisplayOrder))
	for _, name := range platformDisplayOrder {
		platformKey := platformsByDisplayName[name]
		row := volumeByPlatform[platformKey]
		dataSources = append(dataSources, DataSourceStatus{
			Code:   platformShortCode[platformKey],
			Label:  platformSourceLabel[platformKey],
			Value:  formatCompact(row.Value),
			Active: row.Total > 0,
		})
	}

	lastUpdated := "belum ada data"
	if overview.LastUpdated != nil {
		lastUpdated = relativeTimeID(*overview.LastUpdated)
	}

	response.OK(c, i18n.T(lang, "dashboard.overview.get_ok"), DashboardOverviewResponse{
		Headline: DashboardHeadline{
			PositiveShare:      formatPercent1(overview.Positive, overview.Total),
			TotalConversations: totalStr,
			Description:        fmt.Sprintf("Based on %s public conversations about the MBG program.", totalStr),
		},
		Summary: DashboardSummary{
			// change is always "-" — period-over-period comparison isn't
			// implemented yet (needs a defined previous-period window).
			TotalMentions: MetricValueResponse{Value: totalStr, Change: "-"},
			Positive:      MetricValueResponse{Value: formatPercent1(overview.Positive, overview.Total), Change: "-"},
			Negative:      MetricValueResponse{Value: formatPercent1(overview.Negative, overview.Total), Change: "-"},
			Neutral:       MetricValueResponse{Value: formatPercent1(overview.Neutral, overview.Total), Change: "-"},
		},
		Trend:             trend,
		TopicDistribution: topicDistribution,
		TopicTotal:        totalStr,
		KeywordCloud:      keywordCloud,
		RecentMentions:    recentMentions,
		Drivers:           drivers,
		DataSources:       dataSources,
		LastUpdated:       lastUpdated,
	})
}
