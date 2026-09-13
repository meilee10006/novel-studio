package retrieval

import (
	"sort"
	"strings"
	"unicode"
)

type Document struct {
	ID   string
	Text string
}

type scoredDocument struct {
	Document
	score int
	index int
}

func RankKeyword(docs []Document, query string, limit int) []Document {
	if limit <= 0 {
		return nil
	}
	terms := keywordTerms(query)
	if len(terms) == 0 {
		return nil
	}
	ranked := make([]scoredDocument, 0, len(docs))
	for i, doc := range docs {
		text := strings.ToLower(doc.Text)
		score := 0
		for _, term := range terms {
			if strings.Contains(text, term) {
				score++
			}
		}
		if score > 0 {
			ranked = append(ranked, scoredDocument{Document: doc, score: score, index: i})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].index < ranked[j].index
		}
		return ranked[i].score > ranked[j].score
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]Document, len(ranked))
	for i := range ranked {
		out[i] = ranked[i].Document
	}
	return out
}
func keywordTerms(text string) []string {
	var out []string
	var latin []rune
	var han []rune
	flushLatin := func() {
		if len(latin) > 1 {
			out = append(out, strings.ToLower(string(latin)))
		}
		latin = latin[:0]
	}
	flushHan := func() {
		if len(han) == 1 {
			out = append(out, string(han))
		} else {
			for i := 0; i+1 < len(han); i++ {
				out = append(out, string(han[i:i+2]))
			}
		}
		han = han[:0]
	}
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			flushLatin()
			han = append(han, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushHan()
			latin = append(latin, unicode.ToLower(r))
		default:
			flushLatin()
			flushHan()
		}
	}
	flushLatin()
	flushHan()
	seen := map[string]bool{}
	uniq := out[:0]
	for _, term := range out {
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		uniq = append(uniq, term)
	}
	return uniq
}
