package service

import (
	"context"
	"sort"
	"time"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
)

// recentMentionsLimit is how many posts /dashboard/overview's "Recent
// Conversations" panel shows.
const recentMentionsLimit = 5

// topKeywordsLimit is how many words /dashboard/overview's keyword_cloud
// returns.
const topKeywordsLimit = 30

// topDriversLimit is how many themes /dashboard/overview's "Top Sentiment
// Drivers" panel shows.
const topDriversLimit = 5

// DriverStat is one topic category ranked as a "sentiment driver" — its
// mention count (ranking metric and displayed value) and prevailing
// sentiment. "Other" is excluded (see computeDrivers) — it's a leftover
// bucket, not a coherent theme.
type DriverStat struct {
	Category TopicCategory
	Count    int64
	Tone     model.Sentiment
}

// DashboardOverview bundles every aggregate /dashboard/overview needs.
type DashboardOverview struct {
	Total          int64
	Positive       int64
	Negative       int64
	Neutral        int64
	Trend          []repository.DailySentimentRow
	RecentPosts    []model.Post
	PlatformVolume []repository.PlatformVolumeRow
	Keywords       []KeywordCount
	Topics         []TopicCount
	Drivers        []DriverStat
	// LastUpdated is the most recent CrawledAt among all labeled posts, or
	// nil if there are none yet.
	LastUpdated *time.Time
}

type DashboardService interface {
	GetOverview(ctx context.Context, from, to *time.Time, platform string) (DashboardOverview, error)
}

type dashboardService struct {
	postRepo repository.PostRepository
}

func NewDashboardService(postRepo repository.PostRepository) DashboardService {
	return &dashboardService{postRepo: postRepo}
}

// GetOverview aggregates posts AND comments together (Total, sentiment
// breakdown, trend, platform volume, keywords, topics, drivers) — see the
// Combined* repository methods. The one exception is RecentPosts: the
// "Recent Conversations" feed stays post-only by design, since a bare
// comment has no url/location/category of its own and would read as
// context-less floating in that feed.
//
// from/to are nil when the frontend's topbar date-range picker is cleared
// ("view without a date range") — every aggregate below then covers
// all-time. platform is "" when the topbar's
// platform picker is cleared ("All Platforms"). PlatformVolume is the one
// exception — it deliberately ignores platform (see CombinedPlatformVolume's
// doc comment on the repository interface).
func (s *dashboardService) GetOverview(ctx context.Context, from, to *time.Time, platform string) (DashboardOverview, error) {
	agg, err := s.postRepo.CombinedAggregate(ctx, from, to, platform)
	if err != nil {
		return DashboardOverview{}, err
	}

	trend, err := s.postRepo.CombinedTrend(ctx, from, to, platform)
	if err != nil {
		return DashboardOverview{}, err
	}

	recentPosts, _, err := s.postRepo.List(ctx, repository.PostFilter{Platform: platform, From: from, To: to, Page: 1, PerPage: recentMentionsLimit})
	if err != nil {
		return DashboardOverview{}, err
	}

	platformVolume, err := s.postRepo.CombinedPlatformVolume(ctx, from, to)
	if err != nil {
		return DashboardOverview{}, err
	}

	contentRows, err := s.postRepo.CombinedContent(ctx, from, to, platform)
	if err != nil {
		return DashboardOverview{}, err
	}

	texts := make([]string, len(contentRows))
	for i, row := range contentRows {
		texts[i] = row.Content
	}
	keywords := ExtractTopKeywords(texts, topKeywordsLimit)
	topics := ClassifyTopics(texts)
	drivers := computeDrivers(contentRows)

	var lastUpdated *time.Time
	if len(recentPosts) > 0 {
		// List orders by published_at DESC, but crawled_at (when we last
		// ingested it) is the more honest "data freshness" signal — take
		// the max across the page we already fetched rather than adding
		// another query for one timestamp.
		latest := recentPosts[0].CrawledAt
		for _, post := range recentPosts[1:] {
			if post.CrawledAt.After(latest) {
				latest = post.CrawledAt
			}
		}
		lastUpdated = &latest
	}

	return DashboardOverview{
		Total:          agg.Total,
		Positive:       agg.Positive,
		Negative:       agg.FlaggedIssues,
		Neutral:        agg.Neutral,
		Trend:          trend,
		RecentPosts:    recentPosts,
		PlatformVolume: platformVolume,
		Keywords:       keywords,
		Topics:         topics,
		Drivers:        drivers,
		LastUpdated:    lastUpdated,
	}, nil
}

// computeDrivers groups posts by topic category, counts mentions, and picks
// each category's prevailing sentiment (ties favor positive, then negative,
// then neutral). "Other" is skipped — it's a catch-all bucket, not a
// theme worth surfacing as a "driver". Ranked by mention count descending
// (the theme people talk about most), capped at topDriversLimit.
func computeDrivers(rows []repository.PostContentRow) []DriverStat {
	type tally struct {
		category TopicCategory
		count    int64
		positive int64
		negative int64
		neutral  int64
	}
	tallies := make(map[string]*tally)

	for _, row := range rows {
		cat := ClassifyTopic(row.Content)
		if cat.Name == otherTopic.Name {
			continue
		}

		t, ok := tallies[cat.Name]
		if !ok {
			t = &tally{category: cat}
			tallies[cat.Name] = t
		}
		t.count++
		switch row.Sentiment {
		case model.SentimentPositive:
			t.positive++
		case model.SentimentNegative:
			t.negative++
		case model.SentimentNeutral:
			t.neutral++
		}
	}

	result := make([]DriverStat, 0, len(tallies))
	for _, t := range tallies {
		tone := model.SentimentNeutral
		switch {
		case t.positive >= t.negative && t.positive >= t.neutral:
			tone = model.SentimentPositive
		case t.negative >= t.positive && t.negative >= t.neutral:
			tone = model.SentimentNegative
		}
		result = append(result, DriverStat{Category: t.category, Count: t.count, Tone: tone})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	if len(result) > topDriversLimit {
		result = result[:topDriversLimit]
	}
	return result
}
