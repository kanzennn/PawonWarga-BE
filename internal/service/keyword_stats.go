package service

import (
	"sort"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
)

// KeywordStat is one tracked keyword's raw frequency plus its sentiment
// split — the handler derives the dominant sentiment and formats display
// strings from this (same raw-counts-in-service, formatting-in-handler
// split used everywhere else in this codebase).
type KeywordStat struct {
	Text     string
	Count    int64
	Positive int64
	Negative int64
	Neutral  int64
}

// ExtractKeywordStats tokenizes every row's content (see tokenize) and
// tallies each word's frequency plus the sentiment of the post/comment it
// appeared in, returning the topN most frequent, most frequent first.
func ExtractKeywordStats(rows []repository.PostContentRow, topN int) []KeywordStat {
	tallies := make(map[string]*KeywordStat)

	for _, row := range rows {
		for _, token := range tokenize(row.Content) {
			t, ok := tallies[token]
			if !ok {
				t = &KeywordStat{Text: token}
				tallies[token] = t
			}
			t.Count++
			switch row.Sentiment {
			case model.SentimentPositive:
				t.Positive++
			case model.SentimentNegative:
				t.Negative++
			case model.SentimentNeutral:
				t.Neutral++
			}
		}
	}

	result := make([]KeywordStat, 0, len(tallies))
	for _, t := range tallies {
		result = append(result, *t)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Text < result[j].Text // stable tie-break
	})

	if len(result) > topN {
		result = result[:topN]
	}
	return result
}
