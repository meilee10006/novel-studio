package retrieval

import "testing"

func TestRankKeywordPrefersMatchingChineseAndLatinTerms(t *testing.T) {
	docs := []Document{
		{ID: "old", Text: "雨夜里他第一次看见红色纸伞"},
		{ID: "recent", Text: "车站追逐结束，所有人回到旅馆"},
		{ID: "other", Text: "the blue door remains locked"},
	}
	hits := RankKeyword(docs, "红色纸伞", 2)
	if len(hits) == 0 || hits[0].ID != "old" {
		t.Fatalf("hits=%+v", hits)
	}
	latin := RankKeyword(docs, "BLUE door", 1)
	if len(latin) != 1 || latin[0].ID != "other" {
		t.Fatalf("latin=%+v", latin)
	}
}
