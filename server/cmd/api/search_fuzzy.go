package main

import (
	"context"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Search match resolution runs in three stages, from precise to broad:
//
//  1. AND of prefix tokens ("casino* AND royale*") — used when it matches
//     anything, so well-spelled queries stay narrow and fast.
//  2. Typo expansion — each token is expanded with near-spelled terms from the
//     entity's fts5vocab index ("(license* OR \"licence\"*) AND to* AND kill*"),
//     recovering results when a token is misspelled.
//  3. OR of everything — maximum recall fallback.
//
// Stages 2 and 3 only run when the previous stage matched nothing, so the
// common well-spelled case costs the same single count query as before.

// searchVocabMaxCorrections caps how many near-spelled vocabulary terms a
// single query token can expand into.
const searchVocabMaxCorrections = 3

// searchTokens converts a user-supplied query into lowercase FTS-safe tokens.
// Characters that are not letters, digits, or whitespace are dropped so the
// user can't break out of the MATCH expression with FTS5 special syntax
// (quotes, parentheses, AND/OR/NEAR, column filters).
func searchTokens(raw string) []string {
	lowered := strings.ToLower(strings.TrimSpace(raw))
	if lowered == "" {
		return nil
	}

	var b strings.Builder
	b.Grow(len(lowered))
	for _, r := range lowered {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune(' ')
	}
	return strings.Fields(b.String())
}

// renderTokenGroup renders one query token and its vocabulary corrections as
// an FTS5 expression. The token itself is a bare prefix term (it only contains
// letters/digits, per searchTokens); corrections come from the FTS index and
// are quoted string literals with prefix matching.
func renderTokenGroup(group []string) string {
	if len(group) == 1 {
		return group[0] + "*"
	}
	parts := make([]string, len(group))
	parts[0] = group[0] + "*"
	for i, correction := range group[1:] {
		parts[i+1] = `"` + correction + `"*`
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// andPrefixQuery composes token groups into a conjunctive MATCH expression:
// every token (or one of its corrections) must appear in the document.
func andPrefixQuery(groups [][]string) string {
	parts := make([]string, len(groups))
	for i, g := range groups {
		parts[i] = renderTokenGroup(g)
	}
	return strings.Join(parts, " AND ")
}

// orPrefixQuery composes token groups into a disjunctive MATCH expression:
// any token or correction may appear.
func orPrefixQuery(groups [][]string) string {
	var parts []string
	for _, g := range groups {
		parts = append(parts, g[0]+"*")
		for _, correction := range g[1:] {
			parts = append(parts, `"`+correction+`"*`)
		}
	}
	return strings.Join(parts, " OR ")
}

// searchTypoMaxDist returns the maximum edit distance tolerated when
// correcting a query token of the given rune length. Short tokens are not
// corrected at all — nearly everything is one edit away from them.
func searchTypoMaxDist(tokenLen int) int {
	switch {
	case tokenLen < 3:
		return 0
	case tokenLen <= 5:
		return 1
	default:
		return 2
	}
}

// damerauLevenshtein returns the optimal-string-alignment distance between a
// and b (insertions, deletions, substitutions, and adjacent transpositions),
// or maxDist+1 as soon as the distance is known to exceed maxDist.
func damerauLevenshtein(a, b string, maxDist int) int {
	ra := []rune(a)
	rb := []rune(b)
	la, lb := len(ra), len(rb)

	if la-lb > maxDist || lb-la > maxDist {
		return maxDist + 1
	}

	prev2 := make([]int, lb+1)
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d := min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d = min(d, prev2[j-2]+1)
			}
			cur[j] = d
			if d < rowMin {
				rowMin = d
			}
		}
		if rowMin > maxDist {
			return maxDist + 1
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[lb]
}

// vocabCorrections returns up to searchVocabMaxCorrections indexed terms that
// are within typo distance of the query token, nearest first, tie-broken by
// how many documents contain the term. vocabTable always comes from a fixed
// in-code entity spec, never from user input.
func (app *Application) vocabCorrections(ctx context.Context, vocabTable, token string) ([]string, error) {
	tokenLen := utf8.RuneCountInString(token)
	maxDist := searchTypoMaxDist(tokenLen)
	if maxDist == 0 {
		return nil, nil
	}

	// LENGTH() counts characters on SQLite text values, matching the rune
	// count used for the distance band.
	query := "SELECT term, doc FROM " + vocabTable + " WHERE LENGTH(term) BETWEEN ? AND ?"
	rows, err := app.DB.QueryContext(ctx, query, tokenLen-maxDist, tokenLen+maxDist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		term string
		dist int
		doc  int64
	}
	var candidates []candidate
	for rows.Next() {
		var term string
		var doc int64
		err = rows.Scan(&term, &doc)
		if err != nil {
			return nil, err
		}
		if term == token {
			continue
		}
		dist := damerauLevenshtein(token, term, maxDist)
		if dist > maxDist {
			continue
		}
		candidates = append(candidates, candidate{term: term, dist: dist, doc: doc})
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].dist != candidates[j].dist {
			return candidates[i].dist < candidates[j].dist
		}
		if candidates[i].doc != candidates[j].doc {
			return candidates[i].doc > candidates[j].doc
		}
		return candidates[i].term < candidates[j].term
	})
	if len(candidates) > searchVocabMaxCorrections {
		candidates = candidates[:searchVocabMaxCorrections]
	}

	out := make([]string, len(candidates))
	for i, c := range candidates {
		out[i] = c.term
	}
	return out, nil
}

// resolveSearchMatch runs the staged resolution described at the top of this
// file and returns the winning MATCH expression together with its result
// count. ok is false when the query has no usable tokens.
func (app *Application) resolveSearchMatch(ctx context.Context, countSQL, vocabTable, raw string) (match string, total int64, ok bool, err error) {
	tokens := searchTokens(raw)
	if len(tokens) == 0 {
		return "", 0, false, nil
	}

	groups := make([][]string, len(tokens))
	for i, t := range tokens {
		groups[i] = []string{t}
	}

	match = andPrefixQuery(groups)
	total, err = app.searchCount(ctx, countSQL, match)
	if err != nil {
		return "", 0, false, err
	}
	if total > 0 {
		return match, total, true, nil
	}

	expanded := false
	for i, t := range tokens {
		corrections, err := app.vocabCorrections(ctx, vocabTable, t)
		if err != nil {
			return "", 0, false, err
		}
		if len(corrections) > 0 {
			groups[i] = append(groups[i], corrections...)
			expanded = true
		}
	}
	if expanded {
		match = andPrefixQuery(groups)
		total, err = app.searchCount(ctx, countSQL, match)
		if err != nil {
			return "", 0, false, err
		}
		if total > 0 {
			return match, total, true, nil
		}
	}

	match = orPrefixQuery(groups)
	total, err = app.searchCount(ctx, countSQL, match)
	if err != nil {
		return "", 0, false, err
	}
	return match, total, true, nil
}
