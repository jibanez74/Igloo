package spotify

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Match reasons are the values carried by MatchDebugInfo.Reason. The music
// scanner branches on them to decide whether a failed match is "unmatched"
// (definitive) or "failed" (retry later), and they are persisted verbatim to
// music_spotify_matches.reason, so they are exported rather than re-spelled.
const (
	MatchReasonAccepted            = "accepted"
	MatchReasonNoResults           = "no_results"
	MatchReasonScoreBelowThreshold = "score_below_threshold"
	MatchReasonEmptyQuery          = "empty_query"
	MatchReasonSearchFailed        = "search_failed"
	MatchReasonDetailsFailed       = "details_failed"
)

// Lookup and strategy names describe which Spotify query produced a match.
// They are diagnostic only; nothing outside this package branches on them.
const (
	lookupArtist             = "artist"
	lookupAlbum              = "album"
	lookupTrack              = "track"
	strategyArtistSearch     = "artist_search"
	strategyAlbumFieldSearch = "album_field_search"
	strategyAlbumFallback    = "album_fallback_search"
	strategyTrackPosition    = "track_position"
	strategyTrackTitle       = "track_title"
)

const (
	spotifyArtistSearchLimit       = 5
	spotifyAlbumSearchLimit        = 5
	spotifyAlbumRequestSearchLimit = 10
	spotifyTrackRequestSearchLimit = 10
	spotifyArtistThreshold         = 78
	spotifyAlbumThreshold          = 76
	spotifyTrackThreshold          = 70
	// uncreditedAlbumArtistScore stands in for the artist score when a
	// candidate names no artists, or names one that is not the known artist.
	uncreditedAlbumArtistScore = 70
	// artistAdoptionThreshold accepts an album's credited artist as the local
	// artist whose own search missed. It sits below the search threshold
	// because the album match already corroborates the name.
	artistAdoptionThreshold = 60
	// Editions of one album score alike on title and artist; a release year
	// and a track count that agree with the file's tags separate them.
	albumYearBonus       = 3
	albumTrackTotalBonus = 3
	// Two recordings at the same album position within this many
	// milliseconds are the same track however the file titles it.
	trackDurationToleranceMs = 2000
)

// AlbumQuery describes a local album. Artist is the album artist; ArtistID is
// its Spotify id when already known, which makes a candidate crediting that id
// a certain artist match and one crediting a same-named artist without it a
// doubtful one. Year and TotalTracks come from the files' tags and only break
// ties; zero means unknown.
type AlbumQuery struct {
	Title       string
	Artist      string
	ArtistID    ID
	Year        int
	TotalTracks int
}

// TrackQuery describes a local track being paired with one of its album's
// Spotify tracks. Disc and Number are the file's tags, zero when untagged;
// DurationMs is the probed length.
type TrackQuery struct {
	Title      string
	Disc       int
	Number     int
	DurationMs int64
}

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
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, skip := stopWords[field]; skip {
			continue
		}
		tokens = append(tokens, field)
	}

	return tokens
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

	if slices.Equal(queryTokens, candidateTokens) {
		return 98
	}

	if len(queryTokens) == 1 && candidateTokens[0] == queryTokens[0] {
		return 68
	}

	if tokensContainedInOrder(queryTokens, candidateTokens) {
		extraTokens := len(candidateTokens) - len(queryTokens)
		if len(queryTokens) == 1 && extraTokens > 0 {
			return 68
		}
		return 92 - min(extraTokens*6, 18)
	}

	// A single query token can only reach here when it is absent from the
	// candidate, so matched is 0 and this block is a multi-token path.
	matched := countMatchedTokens(queryTokens, candidateTokens)
	if matched == len(queryTokens) {
		extraTokens := len(candidateTokens) - len(queryTokens)
		return 86 - min(extraTokens*5, 20)
	}

	if tokensContainedInOrder(candidateTokens, queryTokens) {
		missingTokens := len(queryTokens) - len(candidateTokens)
		return max(50-missingTokens*10, 0)
	}

	return matched * 60 / len(queryTokens)
}

func scoreTitle(query, candidate string) int {
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
	bestScore = max(bestScore, scoreTokenSequence(baseQueryTokens, baseCandidateTokens))

	if len(baseQueryTokens) > 0 && len(baseCandidateTokens) > 0 && slices.Equal(baseQueryTokens, baseCandidateTokens) {
		bestScore = max(bestScore, 98)
	}

	return bestScore
}

func scoreTokenSequence(queryTokens, candidateTokens []string) int {
	if len(queryTokens) == 0 || len(candidateTokens) == 0 {
		return 0
	}

	if slices.Equal(queryTokens, candidateTokens) {
		return 98
	}

	if tokensContainedInOrder(queryTokens, candidateTokens) {
		extraTokens := len(candidateTokens) - len(queryTokens)
		return 90 - min(extraTokens*4, 16)
	}

	if tokensContainedInOrder(candidateTokens, queryTokens) {
		missingTokens := len(queryTokens) - len(candidateTokens)
		return 84 - min(missingTokens*5, 20)
	}

	matched := countMatchedTokens(queryTokens, candidateTokens)
	if matched == len(queryTokens) {
		extraTokens := len(candidateTokens) - len(queryTokens)
		return 86 - min(extraTokens*4, 16)
	}

	return matched * 70 / len(queryTokens)
}

// scoreAlbumArtist scores a candidate's credits against the query artist. A
// candidate without credits keeps a lenient score so the title can still
// decide; when the artist's Spotify id is known, a candidate crediting it is
// certain and one crediting only a same-named artist is held to that same
// leniency, since it is probably another artist of that name.
func scoreAlbumArtist(query AlbumQuery, candidateArtists []SimpleArtist) (int, string) {
	hasKnownID := query.ArtistID != ""
	if hasKnownID {
		for _, candidateArtist := range candidateArtists {
			if candidateArtist.ID == query.ArtistID {
				return 100, candidateArtist.Name
			}
		}
	}

	if strings.TrimSpace(query.Artist) == "" {
		return 100, ""
	}

	if len(candidateArtists) == 0 {
		return uncreditedAlbumArtistScore, ""
	}

	bestScore := 0
	bestName := ""

	for _, candidateArtist := range candidateArtists {
		score := scoreArtistName(query.Artist, candidateArtist.Name)
		if score <= bestScore {
			continue
		}

		bestScore = score
		bestName = candidateArtist.Name
	}

	if hasKnownID {
		bestScore = min(bestScore, uncreditedAlbumArtistScore)
	}

	return bestScore, bestName
}

func trimAlbumSearchTitle(title string) string {
	searchTitle := strings.TrimSpace(title)
	lowerSearchTitle := strings.ToLower(searchTitle)
	for _, suffix := range []string{" - single", " - ep", " - lp", " - album"} {
		if !strings.HasSuffix(lowerSearchTitle, suffix) {
			continue
		}

		searchTitle = strings.TrimSpace(searchTitle[:len(searchTitle)-len(suffix)])
		break
	}

	return searchTitle
}

func sanitizeSpotifySearchQueryValue(value string) string {
	return strings.ReplaceAll(value, `"`, `'`)
}

func selectBestArtistMatch(query string, artists []FullArtist, strategy string) (*FullArtist, MatchDebugInfo) {
	info := MatchDebugInfo{
		Lookup:      lookupArtist,
		Input:       query,
		SearchQuery: query,
		Strategy:    strategy,
		Threshold:   spotifyArtistThreshold,
		Reason:      MatchReasonNoResults,
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
		info.Reason = MatchReasonScoreBelowThreshold
		return nil, info
	}

	info.Reason = MatchReasonAccepted

	return &artists[bestIndex], info
}

func selectBestAlbumMatch(query AlbumQuery, albums []SimpleAlbum, searchQuery, strategy string) (*SimpleAlbum, MatchDebugInfo) {
	info := MatchDebugInfo{
		Lookup:      lookupAlbum,
		Input:       query.Title,
		SearchQuery: searchQuery,
		Strategy:    strategy,
		Threshold:   spotifyAlbumThreshold,
		Reason:      MatchReasonNoResults,
	}

	if len(albums) == 0 {
		return nil, info
	}

	// Later results pay one point per rank, so a tie keeps Spotify's order.
	hasArtist := strings.TrimSpace(query.Artist) != ""
	bestScore, bestIndex, bestArtistName := math.MinInt, 0, ""
	for index, album := range albums {
		titleScore := scoreTitle(query.Title, album.Name)
		artistScore, candidateArtistName := scoreAlbumArtist(query, album.Artists)
		score := (titleScore*3+artistScore)/4 + albumTagBonus(query, album) - index
		if hasArtist && artistScore < 60 {
			score -= 12
		}
		if score <= bestScore {
			continue
		}

		bestScore, bestIndex, bestArtistName = score, index, candidateArtistName
	}

	info.CandidateName = albums[bestIndex].Name
	info.CandidateArtist = bestArtistName
	info.Score = bestScore

	if bestScore < spotifyAlbumThreshold {
		info.Reason = MatchReasonScoreBelowThreshold
		return nil, info
	}

	info.Reason = MatchReasonAccepted

	return &albums[bestIndex], info
}

func chooseBetterMatchInfo(current, candidate MatchDebugInfo) MatchDebugInfo {
	if candidate.Score > current.Score {
		return candidate
	}

	if candidate.Score == current.Score && current.Reason == MatchReasonNoResults {
		return candidate
	}

	return current
}

func albumTagBonus(query AlbumQuery, album SimpleAlbum) int {
	bonus := 0
	yearAgrees := query.Year > 0 && album.ReleaseDateTime().Year() == query.Year
	if yearAgrees {
		bonus += albumYearBonus
	}
	trackTotalAgrees := query.TotalTracks > 0 && int(album.TotalTracks) == query.TotalTracks
	if trackTotalAgrees {
		bonus += albumTrackTotalBonus
	}
	return bonus
}

// ArtistNamesAgree reports whether candidate plausibly names the artist a
// local credit calls local, loosely enough to adopt an album's credited artist
// when the name search itself missed.
func ArtistNamesAgree(local, candidate string) bool {
	return scoreArtistName(local, candidate) >= artistAdoptionThreshold
}

// MatchAlbumTrack pairs a local track with one of its album's Spotify tracks.
// The file's position nominates a candidate, accepted when its title or its
// length agrees; otherwise the best title wins. The info always carries the
// candidate's title score, even when the length decided.
func MatchAlbumTrack(query TrackQuery, tracks []SimpleTrack) (*SimpleTrack, MatchDebugInfo) {
	info := MatchDebugInfo{
		Lookup:    lookupTrack,
		Input:     query.Title,
		Strategy:  strategyTrackPosition,
		Threshold: spotifyTrackThreshold,
		Reason:    MatchReasonNoResults,
	}

	if len(tracks) == 0 {
		return nil, info
	}

	positioned := trackAtPosition(query, tracks)
	if positioned != nil {
		info.CandidateName = positioned.Name
		info.Score = scoreTitle(query.Title, positioned.Name)
		titleAgrees := info.Score >= spotifyTrackThreshold
		durationAgrees := durationGap(query, *positioned) <= trackDurationToleranceMs
		if titleAgrees || durationAgrees {
			info.Reason = MatchReasonAccepted
			return positioned, info
		}
	}

	info.Strategy = strategyTrackTitle
	bestIndex, bestScore, bestGap := 0, -1, int64(0)
	for index := range tracks {
		score := scoreTitle(query.Title, tracks[index].Name)
		gap := durationGap(query, tracks[index])
		better := score > bestScore || (score == bestScore && gap < bestGap)
		if !better {
			continue
		}
		bestIndex, bestScore, bestGap = index, score, gap
	}

	info.CandidateName = tracks[bestIndex].Name
	info.Score = bestScore
	if bestScore < spotifyTrackThreshold {
		info.Reason = MatchReasonScoreBelowThreshold
		return nil, info
	}

	info.Reason = MatchReasonAccepted
	return &tracks[bestIndex], info
}

// trackAtPosition returns the one Spotify track at the file's disc and track
// number, or nil when the position is untagged or ambiguous (a file with no
// disc tag on a multi-disc album).
func trackAtPosition(query TrackQuery, tracks []SimpleTrack) *SimpleTrack {
	if query.Number <= 0 {
		return nil
	}

	var found *SimpleTrack
	for index := range tracks {
		track := &tracks[index]
		numberAgrees := int(track.TrackNumber) == query.Number
		discAgrees := query.Disc == 0 || int(track.DiscNumber) == query.Disc
		if !numberAgrees || !discAgrees {
			continue
		}
		if found != nil {
			return nil
		}
		found = track
	}

	return found
}

func durationGap(query TrackQuery, track SimpleTrack) int64 {
	gap := int64(track.Duration) - query.DurationMs
	if gap < 0 {
		return -gap
	}
	return gap
}
