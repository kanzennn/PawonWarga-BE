package handler

import (
	"errors"
	"fmt"
	"math"
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

type MentionHandler struct {
	mentionSvc service.MentionService
}

func NewMentionHandler(mentionSvc service.MentionService) *MentionHandler {
	return &MentionHandler{mentionSvc: mentionSvc}
}

// wib is a fixed UTC+7 offset (no DST in Indonesia) — used instead of
// time.LoadLocation("Asia/Jakarta") so formatting doesn't depend on the
// container image shipping IANA tzdata.
var wib = time.FixedZone("WIB", 7*3600)

// DateRangeQuery is the topbar date-range picker's query params, shared
// (via embedding) by every handler that filters on published_at.
type DateRangeQuery struct {
	From string `form:"from"`
	To   string `form:"to"`
}

// parseDateRange parses the topbar date-range picker's `from`/`to` query
// params ("YYYY-MM-DD", WIB) shared by /mentions, /dashboard/overview,
// /sentiment/overview, and /keywords. Either or both may be "" — nil in,
// nil out — matching the picker's "view without a date range" option. `to`
// is inclusive of the whole day (end-of-day 23:59:59.999999999 WIB), so a
// range like from=2026-07-24&to=2026-07-24 covers all of the 24th.
func parseDateRange(fromStr, toStr string) (from, to *time.Time, err error) {
	if fromStr != "" {
		t, parseErr := time.ParseInLocation("2006-01-02", fromStr, wib)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		from = &t
	}
	if toStr != "" {
		t, parseErr := time.ParseInLocation("2006-01-02", toStr, wib)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		endOfDay := t.Add(24*time.Hour - time.Nanosecond)
		to = &endOfDay
	}
	return from, to, nil
}

// platformDisplayNames/platformsByDisplayName/platformDisplayOrder translate
// between the lowercase model.Platform enum and the display strings the
// frontend contract uses ("X", "Instagram", ...) — see
// pawonwarga-fe/docs/api-contract.md.
var platformDisplayNames = map[model.Platform]string{
	model.PlatformX:         "X",
	model.PlatformInstagram: "Instagram",
	model.PlatformTikTok:    "TikTok",
	model.PlatformYouTube:   "YouTube",
	model.PlatformNews:      "News",
}

var platformDisplayOrder = []string{"X", "Instagram", "TikTok", "News", "YouTube"}

var platformsByDisplayName = func() map[string]model.Platform {
	m := make(map[string]model.Platform, len(platformDisplayNames))
	for platform, name := range platformDisplayNames {
		m[name] = platform
	}
	return m
}()

// MentionItem is one entry in /mentions' "items" array and the whole body of
// GET /mentions/{id} — shape fixed by pawonwarga-fe/docs/api-contract.md.
// Location/Category are plain strings, not pointers — the frontend's Mention
// type (lib/types.ts) declares them non-nullable, so a missing value must
// serialize as "" rather than null.
type MentionItem struct {
	ID         uint   `json:"id"`
	Platform   string `json:"platform"`
	User       string `json:"user"`
	Text       string `json:"text"`
	Sentiment  string `json:"sentiment"`
	Engagement string `json:"engagement"`
	Time       string `json:"time"`
	Location   string `json:"location"`
	Category   string `json:"category"`
	Risk       string `json:"risk"`
}

type MentionMetrics struct {
	LiveMentions    int64 `json:"live_mentions"`
	TotalEngagement int64 `json:"total_engagement"`
	// TotalComments is a real count of comments (comments table) matching
	// the same filter as the post list — not to be confused with
	// TotalEngagement, which is like/comment/share counters stored on the
	// posts themselves.
	TotalComments int64 `json:"total_comments"`
	UniqueAuthors int64 `json:"unique_authors"`
	FlaggedIssues int64 `json:"flagged_issues"`
}

// PlatformVolumeItem's Value is engagement (likes+comments+shares stored on
// posts), all-time and unfiltered — MentionCount is the real post+comment
// count matching the current filter. Keep these two straight: Value drives
// the bar width (relative activity volume), MentionCount is the number
// that should be labeled "mentions" in the UI.
type PlatformVolumeItem struct {
	Name         string `json:"name"`
	Value        int64  `json:"value"`
	Positive     int    `json:"positive"`
	Negative     int    `json:"negative"`
	Neutral      int    `json:"neutral"`
	MentionCount int64  `json:"mention_count"`
}

type MentionsListResponse struct {
	Items           []MentionItem        `json:"items"`
	Pagination      response.Pagination  `json:"pagination"`
	Metrics         MentionMetrics       `json:"metrics"`
	PlatformVolume  []PlatformVolumeItem `json:"platform_volume"`
	TotalUnfiltered int64                `json:"total_unfiltered"`
	Platforms       []string             `json:"platforms"`
}

// formatMentionItem maps a labeled model.Post onto the frontend's display
// DTO. Only called with posts that have a non-nil Sentiment — MentionService
// guarantees that (List filters at the DB level, GetPost 404s on unlabeled).
func formatMentionItem(post model.Post) MentionItem {
	user := "Unknown"
	if post.AuthorHandle != nil && *post.AuthorHandle != "" {
		user = "@" + *post.AuthorHandle
	} else if post.AuthorName != nil && *post.AuthorName != "" {
		user = *post.AuthorName
	}

	sentiment := model.SentimentNeutral
	if post.Sentiment != nil {
		sentiment = *post.Sentiment
	}

	engagement := post.LikeCount + post.CommentCount + post.ShareCount

	location := ""
	if post.Location != nil {
		location = *post.Location
	}
	category := ""
	if post.Category != nil {
		category = *post.Category
	}

	return MentionItem{
		ID:         post.ID,
		Platform:   platformDisplayNames[post.Platform],
		User:       user,
		Text:       post.Content,
		Sentiment:  string(sentiment),
		Engagement: formatCompact(int64(engagement)),
		Time:       post.PublishedAt.In(wib).Format("15:04"),
		Location:   location,
		Category:   category,
		Risk:       riskFromSentiment(sentiment),
	}
}

// riskFromSentiment is a simple heuristic (negative -> High, neutral ->
// Medium, positive -> Low), not a distinct backend concept — there is no
// other signal (e.g. engagement-weighted) feeding it yet.
func riskFromSentiment(s model.Sentiment) string {
	switch s {
	case model.SentimentNegative:
		return "High"
	case model.SentimentNeutral:
		return "Medium"
	default:
		return "Low"
	}
}

// formatCompact renders a count the way the dashboard expects ("12.4K"),
// matching the frontend's compact-number display convention.
func formatCompact(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// formatStatValue renders a plain grouped count ("8,275") below 10,000, or a
// formatCompact abbreviation ("13.4K") at or above it — the stat-card
// convention used where a raw count shouldn't be abbreviated until it's
// actually large (e.g. /sentiment/overview's Analyzed Mentions).
func formatStatValue(n int64) string {
	if n < 10_000 {
		return formatGrouped(n)
	}
	return formatCompact(n)
}

func percentOf(part, total int64) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(part) / float64(total) * 100))
}

type ListMentionsQuery struct {
	Query     string `form:"q"`
	Platform  string `form:"platform"`
	Sentiment string `form:"sentiment"`
	// From/To are "YYYY-MM-DD" (WIB) from the topbar date-range picker —
	// both empty means "view without a date range" (all-time).
	From    string `form:"from"`
	To      string `form:"to"`
	Page    int    `form:"page,default=1"`
	PerPage int    `form:"per_page,default=20"`
}

// List godoc
// @Summary      List mentions
// @Description  Lists sentiment-labeled posts for the monitoring dashboard, with metrics and per-platform volume aggregated over the whole filtered set. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         mentions
// @Produce      json
// @Security     BearerAuth
// @Param        q          query     string  false  "Free-text search in content/author"
// @Param        platform   query     string  false  "X, Instagram, TikTok, News, YouTube, or \"All Platforms\""
// @Param        sentiment  query     string  false  "positive, negative, neutral, or \"All Sentiments\""
// @Param        from       query     string  false  "Start date (YYYY-MM-DD, WIB) — omit for no lower bound"
// @Param        to         query     string  false  "End date (YYYY-MM-DD, WIB, inclusive) — omit for no upper bound"
// @Param        page       query     int     false  "Page number"      default(1)
// @Param        per_page   query     int     false  "Items per page"   default(20)
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Router       /mentions [get]
func (h *MentionHandler) List(c *gin.Context) {
	lang := i18n.FromContext(c)

	var query ListMentionsQuery
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
	var sentiment model.Sentiment
	if query.Sentiment != "" && query.Sentiment != "All Sentiments" {
		sentiment = model.Sentiment(strings.ToLower(query.Sentiment))
	}

	page, err := h.mentionSvc.ListMentions(c.Request.Context(), service.ListMentionsInput{
		Platform:  platform,
		Sentiment: sentiment,
		Query:     query.Query,
		From:      from,
		To:        to,
		Page:      query.Page,
		PerPage:   query.PerPage,
	})
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "mentions.posts.list_failed"), err)
		return
	}

	items := make([]MentionItem, len(page.Posts))
	for i, post := range page.Posts {
		items[i] = formatMentionItem(post)
	}

	volumeByPlatform := make(map[model.Platform]repository.PlatformVolumeRow, len(page.PlatformVolume))
	for _, row := range page.PlatformVolume {
		volumeByPlatform[row.Platform] = row
	}

	platformVolume := make([]PlatformVolumeItem, 0, len(platformDisplayOrder))
	for _, name := range platformDisplayOrder {
		platformKey := platformsByDisplayName[name]
		row := volumeByPlatform[platformKey]

		item := PlatformVolumeItem{
			Name:         name,
			Value:        row.Value,
			Positive:     percentOf(row.Positive, row.Total),
			Negative:     percentOf(row.Negative, row.Total),
			Neutral:      percentOf(row.Neutral, row.Total),
			MentionCount: page.PlatformMentionCounts[platformKey],
		}
		platformVolume = append(platformVolume, item)
	}

	response.OK(c, i18n.T(lang, "mentions.posts.list_ok"), MentionsListResponse{
		Items:      items,
		Pagination: response.NewPagination(query.Page, query.PerPage, page.Total),
		Metrics: MentionMetrics{
			LiveMentions:    page.Aggregate.Total,
			TotalEngagement: page.Aggregate.TotalEngagement,
			TotalComments:   page.TotalComments,
			UniqueAuthors:   page.Aggregate.UniqueAuthors,
			FlaggedIssues:   page.Aggregate.FlaggedIssues,
		},
		PlatformVolume:  platformVolume,
		TotalUnfiltered: page.TotalUnfiltered,
		Platforms:       platformDisplayOrder,
	})
}

// GetByID godoc
// @Summary      Get mention detail
// @Description  Returns a single sentiment-labeled mention. 404s if the post exists but hasn't been labeled yet. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         mentions
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Post ID"
// @Success      200  {object}  response.Response
// @Failure      404  {object}  response.ErrorResponse
// @Router       /mentions/{id} [get]
func (h *MentionHandler) GetByID(c *gin.Context) {
	lang := i18n.FromContext(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, i18n.T(lang, "mentions.posts.invalid_id"), err)
		return
	}

	post, err := h.mentionSvc.GetPost(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.NotFound(c, i18n.T(lang, "error.post_not_found"))
			return
		}
		response.InternalServerError(c, i18n.T(lang, "mentions.posts.get_failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "mentions.posts.get_ok"), formatMentionItem(*post))
}
