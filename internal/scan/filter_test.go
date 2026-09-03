package scan

import "testing"

func filterFixture() []Session {
	return []Session{
		{ID: "h1", Title: "fix kafka consumer lag", Cwd: "/repo/backend", Model: "claude-sonnet-5"},
		{ID: "h2", Title: "add webhook retry", Cwd: "/repo/backend", Model: "claude-opus-5"},
		{ID: "r1", Title: "css grid alignment", Cwd: "/repo/frontend", Branch: "feature/css"},
		{ID: "a1", Title: "jvm heap sizing", Cwd: "/elsewhere/ets", Model: "claude-opus-5"},
	}
}

func ids(ss []Session) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.ID
	}
	return out
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name, query string
		want        []string
	}{
		{"empty query keeps all", "", []string{"h1", "h2", "r1", "a1"}},
		{"title substring", "kafka", []string{"h1"}},
		{"case insensitive", "KAFKA", []string{"h1"}},
		{"matches directory", "frontend", []string{"r1"}},
		{"matches branch", "feature/css", []string{"r1"}},
		{"matches model", "sonnet", []string{"h1"}},
		{"terms narrow", "repo retry", []string{"h2"}},
		{"no match", "zzzquux", nil},
		{"fuzzy tolerates a gap", "kafka lag", []string{"h1"}},
		{"typo in title", "kafk", []string{"h1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(Filter(filterFixture(), tc.query))
			if len(got) != len(tc.want) {
				t.Fatalf("Filter(%q) = %v, want %v", tc.query, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("Filter(%q) = %v, want %v", tc.query, got, tc.want)
				}
			}
		})
	}
}

// TestFilterDoesNotMatchAcrossThePath guards the bug where a subsequence search
// over title+path+model let "heap" match "add webhook retry" by picking letters
// out of the directory and the model name.
func TestFilterDoesNotMatchAcrossThePath(t *testing.T) {
	got := ids(Filter(filterFixture(), "heap"))
	if len(got) != 1 || got[0] != "a1" {
		t.Errorf(`Filter("heap") = %v, want only the session actually about heap`, got)
	}
}

func TestWithAncestorsKeepsParentOfMatch(t *testing.T) {
	all := []Session{
		{ID: "child", Title: "forked kafka work", ParentID: "parent"},
		{ID: "parent", Title: "original chat"},
		{ID: "other", Title: "unrelated"},
	}
	matched := Filter(all, "forked")
	got := ids(WithAncestors(all, matched))
	if len(got) != 2 || got[0] != "child" || got[1] != "parent" {
		t.Errorf("WithAncestors = %v, want child then parent (original order)", got)
	}
}

func TestLocalSubsequenceRejectsScatteredLetters(t *testing.T) {
	if localSubsequence("add webhook retry and a pile of other words", "heap") {
		t.Error("matched letters scattered over a long span")
	}
	if !localSubsequence("jvm heap sizing", "heap") {
		t.Error("failed to match a contiguous run")
	}
	if !localSubsequence("kafka consumer lag", "kaflag") {
		t.Error("failed to match a compact subsequence")
	}
	if localSubsequence("kafka consumer lag", "kg") {
		t.Error("a two-letter term should not fuzzy-match")
	}
}
