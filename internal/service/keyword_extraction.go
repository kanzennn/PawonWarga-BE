package service

import (
	"regexp"
	"sort"
	"strings"
)

type KeywordCount struct {
	Text  string
	Count int
}

var (
	tokenPattern   = regexp.MustCompile(`[\p{L}\p{N}]+`)
	urlPattern     = regexp.MustCompile(`https?://\S+`)
	mentionPattern = regexp.MustCompile(`@\w+`)
	stopwords      = buildStopwordSet()
)

func buildStopwordSet() map[string]struct{} {
	words := []string{
		// Indonesian function words / chat slang.
		"yang", "dan", "di", "ke", "dari", "untuk", "dengan", "ini", "itu",
		"pada", "ada", "tidak", "juga", "akan", "atau", "dalam", "adalah",
		"saya", "kita", "kami", "kamu", "dia", "mereka", "nya", "yg", "dg",
		"utk", "tdk", "gak", "ga", "aja", "kalo", "kalau", "kok", "sih",
		"dong", "nih", "deh", "banget", "udah", "sudah", "belum", "masih",
		"lagi", "cuma", "hanya", "seperti", "karena", "bagi", "oleh",
		"sebagai", "secara", "agar", "supaya", "namun", "tapi", "tetapi",
		"jadi", "jika", "apakah", "apa", "siapa", "kenapa", "gimana",
		"bagaimana", "dimana", "kapan", "tentang", "antara", "setelah",
		"sebelum", "hingga", "sampai", "serta", "maupun", "baik", "lebih",
		"kurang", "sangat", "paling",
		// English function words.
		"the", "and", "for", "with", "this", "that", "are", "was", "were",
		"have", "has", "had", "not", "but", "you", "your", "from", "they",
		"them", "their", "what", "who", "when", "where", "why", "how",
		"will", "would", "can", "could", "should", "its", "just", "about",
		"into", "over", "then", "than", "there", "here", "been", "being",
		"some", "such", "only", "also", "very", "much", "more", "most",
		"all", "any", "out", "off", "own", "too", "now",
		// Domain seed keywords — always present by construction (Argus
		// crawls using these as search keywords, see Argus/SKILL.md's
		// nightly crawler defaults), so they'd dominate every result
		// without adding signal.
		"mbg", "sppg", "bgn",
	}

	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// tokenize cleans and splits text into countable words: lowercased, URLs
// and @mentions stripped, then filtered to drop stopwords, numeric-only,
// and sub-3-character tokens. Shared by every keyword-frequency consumer
// (dashboard's keyword_cloud, the standalone /keywords endpoint) so the
// cleaning rules only live in one place.
func tokenize(text string) []string {
	cleaned := urlPattern.ReplaceAllString(text, " ")
	cleaned = mentionPattern.ReplaceAllString(cleaned, " ")
	cleaned = strings.ToLower(cleaned)

	tokens := make([]string, 0)
	for _, token := range tokenPattern.FindAllString(cleaned, -1) {
		if len(token) < 3 || isAllDigits(token) {
			continue
		}
		if _, isStopword := stopwords[token]; isStopword {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// ExtractTopKeywords tokenizes texts and returns the topN most frequent
// words, most frequent first.
func ExtractTopKeywords(texts []string, topN int) []KeywordCount {
	counts := make(map[string]int)
	for _, text := range texts {
		for _, token := range tokenize(text) {
			counts[token]++
		}
	}

	result := make([]KeywordCount, 0, len(counts))
	for text, count := range counts {
		result = append(result, KeywordCount{Text: text, Count: count})
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
