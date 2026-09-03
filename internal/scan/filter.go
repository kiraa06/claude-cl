package scan

import "strings"

// Filter keeps the sessions matching a search query, preserving order.
//
// Every whitespace-separated term must match, so terms narrow the list the way
// they do in a shell. A term matches either as a substring of the whole row —
// title, directory, branch and model — or as a compact subsequence of the
// title, which absorbs a typo or a skipped word without letting letters
// scattered across a long path drag in unrelated sessions.
func Filter(sessions []Session, query string) []Session {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return sessions
	}
	out := make([]Session, 0, len(sessions))
	for _, s := range sessions {
		if matchesAll(s, terms) {
			out = append(out, s)
		}
	}
	return out
}

// haystack is the full text a substring search runs against.
func (s Session) haystack() string {
	return strings.ToLower(s.Title + " " + s.Cwd + " " + s.Branch + " " + s.Model)
}

func matchesAll(s Session, terms []string) bool {
	hay := s.haystack()
	title := strings.ToLower(s.Title)
	for _, t := range terms {
		if !strings.Contains(hay, t) && !localSubsequence(title, t) {
			return false
		}
	}
	return true
}

// localSubsequence reports whether term appears in hay as a subsequence held
// within a short span. The span cap is what keeps the match meaningful: without
// it, any four letters can be found somewhere in a long enough string.
func localSubsequence(hay, term string) bool {
	runes := []rune(term)
	if len(runes) < 3 {
		return false // too short to fuzzy-match without matching everything
	}
	budget := max(len(runes)*3, len(runes)+6)

	h := []rune(hay)
	for start := range h {
		if h[start] != runes[0] {
			continue
		}
		i := 0
		for j := start; j < len(h) && j-start < budget; j++ {
			if h[j] == runes[i] {
				if i++; i == len(runes) {
					return true
				}
			}
		}
	}
	return false
}
