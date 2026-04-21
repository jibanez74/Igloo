package spotify

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	spotifylib "github.com/zmb3/spotify/v2"
	"golang.org/x/text/unicode/norm"
)

const (
	spotifyArtistSearchLimit = 5
	spotifyAlbumSearchLimit  = 5
	spotifyArtistThreshold   = 78
	spotifyAlbumThreshold    = 76
)

var artistStopWords = map[string]struct{}{
	"and": {},
	"the": {},
}

var albumNoiseTokens = map[string]struct{}{
	"album":       {},
	"anniversary": {},
	"bonus":       {},
	"deluxe":      {},
	"edition":     {},
	"ep":          {},
	"expanded":    {},
	"explicit":    {},
	"lp":          {},
	"remaster":    {},
	"remastered":  {},
	"single":      {},
	"version":     {},
}

type MatchDebugInfo struct {
	Lookup          string
	Input           string
	SearchQuery     string
	Strategy        string
	CandidateName   string
	CandidateArtist string
	Score           int
	Threshold       int
	Reason          string
}

type MatchError struct {
	Info MatchDebugInfo
	Err  error
}

func (e *MatchError) Error() string {
	parts := []string{
		fmt.Sprintf("spotify %s match failed", e.Info.Lookup),
		fmt.Sprintf("input=%q", e.Info.Input),
	}

	if e.Info.SearchQuery != "" {
		parts = append(parts, fmt.Sprintf("search=%q", e.Info.SearchQuery))
	}

	if e.Info.CandidateName != "" {
		parts = append(parts, fmt.Sprintf("candidate=%q", e.Info.CandidateName))
	}

	if e.Info.CandidateArtist != "" {
		parts = append(parts, fmt.Sprintf("candidate_artist=%q", e.Info.CandidateArtist))
	}

	if e.Info.Score > 0 || e.Info.Threshold > 0 {
		parts = append(parts, fmt.Sprintf("score=%d", e.Info.Score))
		parts = append(parts, fmt.Sprintf("threshold=%d", e.Info.Threshold))
	}

	if e.Info.Strategy != "" {
		parts = append(parts, fmt.Sprintf("strategy=%s", e.Info.Strategy))
	}

	if e.Info.Reason != "" {
		parts = append(parts, fmt.Sprintf("reason=%s", e.Info.Reason))
	}

	if e.Err != nil {
		parts = append(parts, fmt.Sprintf("error=%v", e.Err))
	}

	return strings.Join(parts, " ")
}

func (e *MatchError) Unwrap() error {
	return e.Err
}

func AsMatchError(err error) (*MatchError, bool) {
	var matchErr *MatchError
	if errors.As(err, &matchErr) {
		return matchErr, true
	}

	return nil, false
}

func newMatchError(info MatchDebugInfo, err error) error {
	return &MatchError{
		Info: info,
		Err:  err,
	}
}

func normalizeComparisonText(value string) string {
	decomposed := norm.NFD.String(strings.TrimSpace(value))
	var builder strings.Builder
	lastWasSpace := true

	for _, r := range decomposed {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r == '&' || r == '+':
			if !lastWasSpace {
				builder.WriteByte(' ')
			}
			builder.WriteString("and")
			builder.WriteByte(' ')
			lastWasSpace = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
			lastWasSpace = false
		case unicode.IsSpace(r):
			if !lastWasSpace {
				builder.WriteByte(' ')
				lastWasSpace = true
			}
		default:
			if !lastWasSpace {
				builder.WriteByte(' ')
				lastWasSpace = true
			}
		}
	}

	return strings.Join(strings.Fields(builder.String()), " ")
}

func tokenizeComparisonText(value string, stopWords map[string]struct{}) []string {
	normalized := normalizeComparisonText(value)
	if normalized == "" {
		return nil
	}

	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, skip := stopWords[field]; skip {
			continue
		}
		tokens = append(tokens, field)
	}

	return tokens
}

func tokensEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

func tokensContainedInOrder(queryTokens, candidateTokens []string) bool {
	if len(queryTokens) == 0 || len(candidateTokens) == 0 || len(queryTokens) > len(candidateTokens) {
		return false
	}

	queryIndex := 0
	for _, candidateToken := range candidateTokens {
		if candidateToken != queryTokens[queryIndex] {
			continue
		}

		queryIndex++
		if queryIndex == len(queryTokens) {
			return true
		}
	}

	return false
}

func countMatchedTokens(queryTokens, candidateTokens []string) int {
	if len(queryTokens) == 0 || len(candidateTokens) == 0 {
		return 0
	}

	counts := make(map[string]int, len(candidateTokens))
	for _, token := range candidateTokens {
		counts[token]++
	}

	matched := 0
	for _, token := range queryTokens {
		if counts[token] == 0 {
			continue
		}

		counts[token]--
		matched++
	}

	return matched
}

func scoreArtistName(query, candidate string) int {
	normalizedQuery := normalizeComparisonText(query)
	normalizedCandidate := normalizeComparisonText(candidate)
	if normalizedQuery == "" || normalizedCandidate == "" {
		return 0
	}

	if normalizedQuery == normalizedCandidate {
		return 100
	}

	queryTokens := tokenizeComparisonText(query, artistStopWords)
	candidateTokens := tokenizeComparisonText(candidate, artistStopWords)
	if len(queryTokens) == 0 || len(candidateTokens) == 0 {
		return 0
	}

	if tokensEqual(queryTokens, candidateTokens) {
		return 98
	}

	if len(queryTokens) == 1 {
		if len(candidateTokens) == 1 && queryTokens[0] == candidateTokens[0] {
			return 100
		}
		if candidateTokens[0] == queryTokens[0] {
			return 68
		}
	}

	if tokensContainedInOrder(queryTokens, candidateTokens) {
		extraTokens := len(candidateTokens) - len(queryTokens)
		score := 92 - minInt(extraTokens*6, 18)
		if len(queryTokens) == 1 && extraTokens > 0 {
			score = 68
		}
		return maxInt(score, 0)
	}

	matched := countMatchedTokens(queryTokens, candidateTokens)
	if matched == len(queryTokens) {
		extraTokens := len(candidateTokens) - len(queryTokens)
		score := 86 - minInt(extraTokens*5, 20)
		if len(queryTokens) == 1 && extraTokens > 0 {
			score = 68
		}
		return maxInt(score, 0)
	}

	if tokensContainedInOrder(candidateTokens, queryTokens) {
		missingTokens := len(queryTokens) - len(candidateTokens)
		return maxInt(50-missingTokens*10, 0)
	}

	return matched * 60 / len(queryTokens)
}

func scoreAlbumTitle(query, candidate string) int {
	normalizedQuery := normalizeComparisonText(query)
	normalizedCandidate := normalizeComparisonText(candidate)
	if normalizedQuery == "" || normalizedCandidate == "" {
		return 0
	}

	if normalizedQuery == normalizedCandidate {
		return 100
	}

	fullQueryTokens := tokenizeComparisonText(query, nil)
	fullCandidateTokens := tokenizeComparisonText(candidate, nil)
	baseQueryTokens := tokenizeComparisonText(query, albumNoiseTokens)
	baseCandidateTokens := tokenizeComparisonText(candidate, albumNoiseTokens)

	bestScore := scoreTokenSequence(fullQueryTokens, fullCandidateTokens)
	bestScore = maxInt(bestScore, scoreTokenSequence(baseQueryTokens, baseCandidateTokens))

	if len(baseQueryTokens) > 0 && len(baseCandidateTokens) > 0 && tokensEqual(baseQueryTokens, baseCandidateTokens) {
		bestScore = maxInt(bestScore, 98)
	}

	return bestScore
}

func scoreTokenSequence(queryTokens, candidateTokens []string) int {
	if len(queryTokens) == 0 || len(candidateTokens) == 0 {
		return 0
	}

	if tokensEqual(queryTokens, candidateTokens) {
		return 98
	}

	if tokensContainedInOrder(queryTokens, candidateTokens) {
		extraTokens := len(candidateTokens) - len(queryTokens)
		return maxInt(90-minInt(extraTokens*4, 16), 0)
	}

	if tokensContainedInOrder(candidateTokens, queryTokens) {
		missingTokens := len(queryTokens) - len(candidateTokens)
		return maxInt(84-minInt(missingTokens*5, 20), 0)
	}

	matched := countMatchedTokens(queryTokens, candidateTokens)
	if matched == len(queryTokens) {
		extraTokens := len(candidateTokens) - len(queryTokens)
		return maxInt(86-minInt(extraTokens*4, 16), 0)
	}

	return matched * 70 / len(queryTokens)
}

func scoreAlbumArtist(queryArtist string, candidateArtists []spotifylib.SimpleArtist) (int, string) {
	if strings.TrimSpace(queryArtist) == "" {
		return 100, ""
	}

	if len(candidateArtists) == 0 {
		return 70, ""
	}

	bestScore := 0
	bestName := ""

	for _, candidateArtist := range candidateArtists {
		score := scoreArtistName(queryArtist, candidateArtist.Name)
		if score <= bestScore {
			continue
		}

		bestScore = score
		bestName = candidateArtist.Name
	}

	return bestScore, bestName
}

func trimAlbumSearchTitle(title string) string {
	searchTitle := strings.TrimSpace(title)
	for _, suffix := range []string{" - single", " - ep", " - lp", " - album"} {
		index := strings.Index(strings.ToLower(searchTitle), suffix)
		if index == -1 {
			continue
		}

		searchTitle = strings.TrimSpace(searchTitle[:index])
		break
	}

	return searchTitle
}

func selectBestArtistMatch(query string, artists []spotifylib.FullArtist, strategy string) (*spotifylib.FullArtist, MatchDebugInfo) {
	info := MatchDebugInfo{
		Lookup:      "artist",
		Input:       query,
		SearchQuery: query,
		Strategy:    strategy,
		Threshold:   spotifyArtistThreshold,
		Reason:      "no_results",
	}

	if len(artists) == 0 {
		return nil, info
	}

	bestIndex := -1
	bestScore := -1

	for index := range artists {
		score := scoreArtistName(query, artists[index].Name) - index
		if score <= bestScore {
			continue
		}

		bestScore = score
		bestIndex = index
	}

	info.CandidateName = artists[bestIndex].Name
	info.Score = bestScore

	if bestScore < spotifyArtistThreshold {
		info.Reason = "score_below_threshold"
		return nil, info
	}

	info.Reason = "accepted"

	return &artists[bestIndex], info
}

func selectBestAlbumMatch(title, artist string, albums []spotifylib.SimpleAlbum, searchQuery, strategy string) (*spotifylib.SimpleAlbum, MatchDebugInfo) {
	info := MatchDebugInfo{
		Lookup:      "album",
		Input:       title,
		SearchQuery: searchQuery,
		Strategy:    strategy,
		Threshold:   spotifyAlbumThreshold,
		Reason:      "no_results",
	}

	if len(albums) == 0 {
		return nil, info
	}

	bestIndex := -1
	bestScore := -1
	bestArtistName := ""

	for index := range albums {
		titleScore := scoreAlbumTitle(title, albums[index].Name)
		artistScore, candidateArtistName := scoreAlbumArtist(artist, albums[index].Artists)
		score := (titleScore*3 + artistScore) / 4
		if strings.TrimSpace(artist) != "" && artistScore < 60 {
			score -= 12
		}
		score -= index

		if score <= bestScore {
			continue
		}

		bestScore = score
		bestIndex = index
		bestArtistName = candidateArtistName
	}

	info.CandidateName = albums[bestIndex].Name
	info.CandidateArtist = bestArtistName
	info.Score = bestScore

	if bestScore < spotifyAlbumThreshold {
		info.Reason = "score_below_threshold"
		return nil, info
	}

	info.Reason = "accepted"

	return &albums[bestIndex], info
}

func chooseBetterMatchInfo(current, candidate MatchDebugInfo) MatchDebugInfo {
	if candidate.Score > current.Score {
		return candidate
	}

	if candidate.Score == current.Score && current.Reason == "no_results" {
		return candidate
	}

	return current
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}

	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}

	return right
}
