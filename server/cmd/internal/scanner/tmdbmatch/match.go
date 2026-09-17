// Package tmdbmatch ranks TMDB search results against a locally parsed title
// and year. The movie and TV scanners and the manual identify handler share
// it so one scoring rule decides every automatic match.
package tmdbmatch

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
)

const yearMatchScore = 20.0

// Match is one scored TMDB candidate.
type Match struct {
	Movie      *tmdb.TmdbMovie
	Score      float64
	Confidence float64
}

// Interpretation is one reading of a filename or folder name: the title to
// search and the year to prefer, or 0 when the name carries none.
type Interpretation struct {
	Title string
	Year  int
}

// Candidates keeps the best Match per TMDB id across several search
// interpretations, so a result returned by two searches is scored once by its
// strongest reading.
type Candidates struct {
	best map[int]*Match
}

// Add records every ranked match whose id is valid, replacing a weaker entry
// for the same id.
func (c *Candidates) Add(ranked []*Match) {
	if c.best == nil {
		c.best = make(map[int]*Match)
	}
	for _, match := range ranked {
		id := match.Movie.TmdbID
		if id <= 0 {
			continue
		}
		previous, exists := c.best[id]
		better := !exists || Compare(match, previous) < 0
		if better {
			c.best[id] = match
		}
	}
}

// Best returns the top candidate, or nil when nothing was added. Ties are
// deterministic: score, then title, then TMDB id.
func (c *Candidates) Best() *Match {
	ranked := make([]*Match, 0, len(c.best))
	for _, candidate := range c.best {
		ranked = append(ranked, candidate)
	}
	slices.SortFunc(ranked, Compare)
	if len(ranked) == 0 {
		return nil
	}
	return ranked[0]
}

// Compare orders matches best first with deterministic ties.
func Compare(a, b *Match) int {
	if a.Score != b.Score {
		return cmp.Compare(b.Score, a.Score)
	}
	if a.Movie.Title != b.Movie.Title {
		return strings.Compare(a.Movie.Title, b.Movie.Title)
	}
	return cmp.Compare(a.Movie.TmdbID, b.Movie.TmdbID)
}

// ReleaseYear extracts the year from a TMDB release or first-air date.
func ReleaseYear(releaseDate string) int {
	if releaseDate == "" {
		return 0
	}

	parsed, err := helpers.ParseDate(releaseDate)
	if err != nil {
		return 0
	}

	return parsed.Year()
}

// Rank orders TMDB results by the scanner's title and year score.
func Rank(results []tmdb.TmdbMovie, targetTitle string, targetYear int) []*Match {
	if len(results) == 0 {
		return nil
	}

	normalizedTarget := normalizeComparableTitle(targetTitle)
	targetSequel := sequelIndicator(normalizedTarget)

	scoredMatches := make([]*Match, 0, len(results))
	for i := range results {
		movie := &results[i]
		score := scoreCandidate(normalizedTarget, targetSequel, targetYear, movie)
		scoredMatches = append(scoredMatches, &Match{
			Movie:      movie,
			Score:      score,
			Confidence: clampConfidence(score),
		})
	}

	slices.SortFunc(scoredMatches, Compare)

	return scoredMatches
}

// scoreCandidate scores one candidate against an already-normalized target
// title and its sequel indicator, both of which are loop-invariant across a
// ranking pass.
func scoreCandidate(normalizedTarget, targetSequel string, targetYear int, movie *tmdb.TmdbMovie) float64 {
	normalizedTitle := normalizeComparableTitle(movie.Title)
	normalizedOriginalTitle := normalizeComparableTitle(movie.OriginalTitle)

	score := 0.0

	switch {
	case normalizedTitle == normalizedTarget:
		score += 60
	case normalizedOriginalTitle == normalizedTarget:
		score += 56
	case normalizedTarget != "" && (strings.Contains(normalizedTitle, normalizedTarget) || strings.Contains(normalizedTarget, normalizedTitle)):
		score += 38
	}

	score += tokenOverlapScore(normalizedTarget, normalizedTitle) * 35
	score += tokenOverlapScore(normalizedTarget, normalizedOriginalTitle) * 20

	if targetSequel != "" && targetSequel == sequelIndicator(normalizedTitle) {
		score += 8
	} else if targetSequel != "" {
		score -= 12
	}

	movieYear := ReleaseYear(movie.ReleaseDate)
	if targetYear > 0 {
		switch {
		case movieYear == targetYear:
			score += yearMatchScore
		case movieYear > 0 && absInt(movieYear-targetYear) == 1:
			score += 12
		case movieYear > 0:
			score -= 15
		}
	}

	score += min(movie.Popularity/25, 8)
	score += min(movie.VoteAverage/2, 5)

	return score
}

// Replacers are immutable and build a trie on construction, so they are built
// once here rather than per call: ranking a 20-result TMDB search normalizes
// titles dozens of times per scanned file.
var (
	audioLayoutReplacer    = strings.NewReplacer("5.1", " ", "7.1", " ", "2.0", " ")
	titleSeparatorReplacer = strings.NewReplacer(".", " ", "_", " ", "-", " ", "(", " ", ")", " ", "[", " ", "]", " ")
)

// NormalizeTitleForSearch removes filename release noise before a TMDB search.
func NormalizeTitleForSearch(title string) string {
	title = audioLayoutReplacer.Replace(title)
	normalized := titleSeparatorReplacer.Replace(strings.ToLower(strings.TrimSpace(title)))
	tokens := strings.Fields(normalized)
	cleaned := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.Trim(token, ".,!?:;'+\"")
		token = strings.ReplaceAll(token, "'", "")
		token = strings.ReplaceAll(token, "-", "")
		if token == "" {
			continue
		}
		if helpers.IsMovieReleaseNoiseToken(token) {
			continue
		}
		if isBracketedReleaseGroupToken(token) {
			continue
		}
		if isTechnicalToken(token) {
			continue
		}
		cleaned = append(cleaned, token)
	}
	return strings.Join(cleaned, " ")
}

func normalizeComparableTitle(title string) string {
	title = NormalizeTitleForSearch(title)
	var b strings.Builder
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func isBracketedReleaseGroupToken(token string) bool {
	return strings.HasPrefix(token, "yts") || strings.HasPrefix(token, "rarbg")
}

func isTechnicalToken(token string) bool {
	if token == "aac" || strings.HasPrefix(token, "x26") || strings.HasPrefix(token, "h26") {
		return true
	}

	bitDepth := strings.TrimSuffix(token, "bit")
	if bitDepth != token && len(bitDepth) >= 1 && len(bitDepth) <= 2 {
		for _, digit := range bitDepth {
			if digit < '0' || digit > '9' {
				return false
			}
		}
		return true
	}

	return false
}

func tokenOverlapScore(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	aTokens := strings.Fields(a)
	bTokens := strings.Fields(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}

	seen := make(map[string]bool)
	for _, token := range aTokens {
		seen[token] = true
	}

	matches := 0
	for _, token := range bTokens {
		if seen[token] {
			matches++
		}
	}

	denominator := max(len(aTokens), len(bTokens))
	return float64(matches) / float64(denominator)
}

func clampConfidence(score float64) float64 {
	switch {
	case score < 0:
		return 0
	case score > 100:
		return 100
	default:
		return score
	}
}

func sequelIndicator(title string) string {
	tokens := strings.Fields(title)
	for _, token := range tokens {
		switch token {
		case "2", "ii", "3", "iii", "4", "iv", "5", "v":
			return token
		}
	}
	return ""
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
