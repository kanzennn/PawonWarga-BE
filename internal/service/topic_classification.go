package service

import (
	"strings"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
)

// TopicCategory is one slice of the /dashboard/overview topic_distribution
// donut — a name plus the color the frontend renders it in (backend owns
// the color so the donut and its legend always agree, per
// pawonwarga-fe/docs/api-contract.md).
type TopicCategory struct {
	Name  string
	Color string
}

type topicDefinition struct {
	TopicCategory
	Keywords []string
}

// topicDefinitions is a hand-picked taxonomy for MBG monitoring, derived
// from patterns observed in real ingested posts (kitchen/SPPG operations,
// transparency sidak, nutrition benefit claims, policy/BGN mentions,
// misuse allegations). Keyword matching is case-insensitive substring —
// crude but transparent and cheap to tune; revisit if/when a real
// taxonomy or LLM-based classifier is wanted instead.
var topicDefinitions = []topicDefinition{
	{
		TopicCategory{"Kualitas & Keamanan Pangan", "#0c8a5f"},
		[]string{
			"kualitas", "mutu", "higienis", "higiene", "kebersihan",
			"keracunan", "basi", "busuk", "kadaluarsa", "kadaluwarsa",
			"standar", "sertifikasi", "layak",
		},
	},
	{
		TopicCategory{"Distribusi & Operasional Dapur", "#2f6f9e"},
		[]string{
			"dapur", "sppg", "distribusi", "logistik", "pengiriman",
			"memasak", "produksi", "operasional", "armada", "antar",
			"relawan",
		},
	},
	{
		TopicCategory{"Manfaat Gizi", "#67c74a"},
		[]string{
			"gizi", "nutrisi", "manfaat", "tumbuh", "kembang", "stunting",
			"vitamin", "protein", "cerdas", "prestasi", "konsentrasi",
		},
	},
	{
		TopicCategory{"Kebijakan & Transparansi", "#f4b21b"},
		[]string{
			"kebijakan", "transparansi", "anggaran", "bgn",
			"badan gizi nasional", "regulasi", "program", "pemerintah",
			"presiden", "prabowo", "sudaryono", "evaluasi", "audit", "kdmp",
		},
	},
	{
		TopicCategory{"Dugaan Penyimpangan", "#e54848"},
		[]string{
			"korupsi", "penyimpangan", "penyelewengan", "dugaan", "skandal",
			"penyalahgunaan", "markup", "mark up", "suap", "oknum",
			"penipuan", "sidak",
		},
	},
}

var otherTopic = TopicCategory{"Lainnya", "#9aa39c"}

// ClassifyTopic assigns content to whichever category has the most keyword
// hits (case-insensitive substring match, ties favor the earlier-listed
// category). Falls back to "Lainnya" if nothing matches.
func ClassifyTopic(content string) TopicCategory {
	lower := strings.ToLower(content)

	bestScore := 0
	best := otherTopic

	for _, def := range topicDefinitions {
		score := 0
		for _, kw := range def.Keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = def.TopicCategory
		}
	}

	return best
}

type TopicCount struct {
	Category TopicCategory
	Count    int
}

// ClassifyTopics classifies every text and returns per-category counts in a
// stable order (topicDefinitions' order, "Lainnya" last), omitting any
// category with zero matches so the donut doesn't carry empty slices.
func ClassifyTopics(texts []string) []TopicCount {
	counts := make(map[string]int, len(topicDefinitions)+1)
	for _, text := range texts {
		counts[ClassifyTopic(text).Name]++
	}

	order := make([]TopicCategory, 0, len(topicDefinitions)+1)
	for _, def := range topicDefinitions {
		order = append(order, def.TopicCategory)
	}
	order = append(order, otherTopic)

	result := make([]TopicCount, 0, len(order))
	for _, cat := range order {
		if count := counts[cat.Name]; count > 0 {
			result = append(result, TopicCount{Category: cat, Count: count})
		}
	}
	return result
}

// CategorySentimentStat is one topic category's sentiment breakdown — raw
// counts, not percentages (the handler converts, same as everywhere else in
// this codebase). Backs /sentiment/overview's by_category chart.
type CategorySentimentStat struct {
	Category TopicCategory
	Positive int64
	Negative int64
	Neutral  int64
	Total    int64
}

// ClassifyTopicSentiment groups rows by topic and tallies sentiment within
// each. "Lainnya" is excluded — same rationale as computeDrivers, it's a
// catch-all bucket, not a theme worth charting.
func ClassifyTopicSentiment(rows []repository.PostContentRow) []CategorySentimentStat {
	tallies := make(map[string]*CategorySentimentStat)

	for _, row := range rows {
		cat := ClassifyTopic(row.Content)
		if cat.Name == otherTopic.Name {
			continue
		}

		t, ok := tallies[cat.Name]
		if !ok {
			t = &CategorySentimentStat{Category: cat}
			tallies[cat.Name] = t
		}
		t.Total++
		switch row.Sentiment {
		case model.SentimentPositive:
			t.Positive++
		case model.SentimentNegative:
			t.Negative++
		case model.SentimentNeutral:
			t.Neutral++
		}
	}

	order := make([]TopicCategory, 0, len(topicDefinitions))
	for _, def := range topicDefinitions {
		order = append(order, def.TopicCategory)
	}

	result := make([]CategorySentimentStat, 0, len(tallies))
	for _, cat := range order {
		if t, ok := tallies[cat.Name]; ok {
			result = append(result, *t)
		}
	}
	return result
}
