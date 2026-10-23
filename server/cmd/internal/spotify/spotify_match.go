package spotify

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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
	MatchReasonTrackClaimed        = "track_claimed"
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
	// A release with fewer than half the tracks the file's tags count is a
	// single or EP from the album, not the album; the penalty outweighs both
	// bonuses and a few ranks of Spotify's order.
	albumSizePenalty = 6
	// Two recordings at the same album position within this many
	// milliseconds are the same track however the file titles it.
	trackDurationToleranceMs = 2000
)

// AlbumQuery describes a local album. Artist is the album artist; ArtistID is
// its Spotify id when already known, which makes a candidate crediting that id
// a certain artist match and one crediting a same-named artist without it a
// doubtful one. ArtistCredits are the names a compound album artist credits,
// such as "A, B & C", each of which may be the one Spotify credits. Year and
// TotalTracks come from the files' tags; they break ties between editions,
// and TotalTracks also marks down a release too small to be the album. Zero
// means unknown.
type AlbumQuery struct {
	Title         string
	Artist        string
	ArtistCredits []string
	ArtistID      ID
	Year          int
	TotalTracks   int
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

// HeldTrack is a local track already paired with the Spotify track ID, which
// no other file of its album may take.
type HeldTrack struct {
	TrackQuery
	ID ID
}

// TrackPairing is one local track's outcome: the Spotify track it pairs with,
// nil on a miss, and the diagnostics that explain either.
type TrackPairing struct {
	Track *SimpleTrack
	Info  MatchDebugInfo
}

var artistStopWords = map[string]struct{}{
	"and": {},
	"the": {},
}

// comparisonTokenAliases spell abbreviations out, so "Vol. One" compares
// equal to "Volume One".
var comparisonTokenAliases = map[string]string{
	"vol": "volume",
}

// variousArtists is how compilations credit their artist, locally and on
// Spotify; it names no artist, so it cannot confirm or deny one.
const variousArtists = "various artists"

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

// trackVersionWords name a different recording when a track title's
// qualifier carries one the other title lacks: "Orgullecida (Alternate Take)"
// is not "Orgullecida", while "Live and Let Die" carries no qualifier at all.
var trackVersionWords = map[string]struct{}{
	"acoustic":     {},
	"alternate":    {},
	"demo":         {},
	"finale":       {},
	"instrumental": {},
	"karaoke":      {},
	"live":         {},
	"mix":          {},
	"remix":        {},
	"reprise":      {},
	"take":         {},
	"unplugged":    {},
}

// trackRemasterWords make up a qualifier that only notes a remaster, as in
// Spotify's "Song - Remastered 2011"; a four-digit year counts too.
var trackRemasterWords = map[string]struct{}{
	"digital":    {},
	"digitally":  {},
	"remaster":   {},
	"remastered": {},
	"version":    {},
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
		if alias, ok := comparisonTokenAliases[field]; ok {
			field = alias
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

// scoreTrackTitle scores a track title the way scoreTitle does, and also
// compares both titles' cores, without featured artists, source notes or
// remaster notes, keeping the better score. A version word in only one title
// keeps the score below the threshold.
func scoreTrackTitle(query, candidate string) int {
	queryCore, queryVersions := trackTitleCore(query)
	candidateCore, candidateVersions := trackTitleCore(candidate)
	score := max(scoreTitle(query, candidate), scoreTitle(queryCore, candidateCore))
	if !maps.Equal(queryVersions, candidateVersions) {
		score = min(score, spotifyTrackThreshold-1)
	}
	return score
}

// scoreAlbumTitle scores an album title the way scoreTitle does, and also
// without the qualifiers that note a remaster, keeping the better score:
// "Virtuoso (OJC Remaster)" is "Virtuoso (Remastered 2025)".
func scoreAlbumTitle(query, candidate string) int {
	return max(scoreTitle(query, candidate), scoreTitle(albumTitleCore(query), albumTitleCore(candidate)))
}

// albumTitleCore drops every bracketed or " - " qualifier that mentions a
// remaster. Only the remaster words count: "version" alone would also drop
// "(Taylor's Version)", which names a different album.
func albumTitleCore(title string) string {
	base, qualifiers := titleParts(title)
	core := []string{base}
	for _, qualifier := range qualifiers {
		words := strings.Fields(normalizeComparisonText(qualifier))
		if !slices.Contains(words, "remaster") && !slices.Contains(words, "remastered") {
			core = append(core, qualifier)
		}
	}
	return strings.Join(core, " ")
}

// trackTitleCore drops the qualifiers that name no different recording and
// returns what remains, with the version words the kept qualifiers carry.
func trackTitleCore(title string) (string, map[string]struct{}) {
	base, qualifiers := titleParts(title)
	core := []string{base}
	versions := map[string]struct{}{}
	for _, qualifier := range qualifiers {
		words := strings.Fields(normalizeComparisonText(qualifier))
		if trackQualifierDropped(words) {
			continue
		}
		core = append(core, qualifier)
		for _, word := range words {
			_, isVersion := trackVersionWords[word]
			if isVersion {
				versions[word] = struct{}{}
			}
		}
	}
	return strings.Join(core, " "), versions
}

// titleParts splits a title into the text outside brackets and its
// qualifiers: each bracketed segment, and everything after a " - " outside
// brackets, as in "Song (feat. X) - Remastered 2011".
func titleParts(title string) (string, []string) {
	var base, segment strings.Builder
	var qualifiers []string
	depth := 0
	for i := 0; i < len(title); {
		if depth == 0 && strings.HasPrefix(title[i:], " - ") {
			qualifiers = append(qualifiers, title[i+len(" - "):])
			break
		}
		r, size := utf8.DecodeRuneInString(title[i:])
		i += size
		switch {
		case r == '(' || r == '[':
			if depth > 0 {
				segment.WriteRune(r)
			}
			depth++
		case (r == ')' || r == ']') && depth > 0:
			depth--
			if depth == 0 {
				qualifiers = append(qualifiers, segment.String())
				segment.Reset()
			} else {
				segment.WriteRune(r)
			}
		case depth > 0:
			segment.WriteRune(r)
		default:
			base.WriteRune(r)
		}
	}
	if segment.Len() > 0 {
		qualifiers = append(qualifiers, segment.String())
	}
	return base.String(), qualifiers
}

// trackQualifierDropped reports a qualifier that names no different
// recording: a featured artist, a "From ..." source note, or a remaster note.
func trackQualifierDropped(words []string) bool {
	if len(words) == 0 {
		return true
	}
	switch words[0] {
	case "feat", "ft", "featuring", "with", "from":
		return true
	}
	for _, word := range words {
		_, isRemaster := trackRemasterWords[word]
		isYear := len(word) == 4 && strings.Trim(word, "0123456789") == ""
		if !isRemaster && !isYear {
			return false
		}
	}
	return true
}

// scoreAlbumArtist scores a candidate's credits against the query artist and
// each of its credits. A candidate without credits keeps a lenient score so
// the title can still decide, and so does a side credited to "Various
// Artists" against a named artist; when the artist's Spotify id is known, a
// candidate crediting it is certain and one crediting only a same-named artist
// is held to that same leniency, since it is probably another artist of that
// name.
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
		for _, credit := range query.ArtistCredits {
			score = max(score, scoreArtistName(credit, candidateArtist.Name))
		}
		if score <= bestScore {
			continue
		}

		bestScore = score
		bestName = candidateArtist.Name
	}

	if hasKnownID {
		bestScore = min(bestScore, uncreditedAlbumArtistScore)
	}
	if isVariousArtists(query.Artist) || slices.ContainsFunc(candidateArtists, func(artist SimpleArtist) bool { return isVariousArtists(artist.Name) }) {
		bestScore = max(bestScore, uncreditedAlbumArtistScore)
	}

	return bestScore, bestName
}

func isVariousArtists(name string) bool {
	return normalizeComparisonText(name) == variousArtists
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
		titleScore := scoreAlbumTitle(query.Title, album.Name)
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

// albumTagBonus weighs a candidate against the file's tags: a release year and
// a track count that agree add to its score, and a release too small to be
// the tagged album takes away.
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
	if int(album.TotalTracks)*2 < query.TotalTracks {
		bonus -= albumSizePenalty
	}
	return bonus
}

// ArtistNamesAgree reports whether candidate plausibly names the artist a
// local credit calls local, loosely enough to adopt an album's credited artist
// when the name search itself missed.
func ArtistNamesAgree(local, candidate string) bool {
	return scoreArtistName(local, candidate) >= artistAdoptionThreshold
}

// trackEdge is one acceptable pairing of a pending local track with a
// Spotify track. tier ranks its evidence: the file's position with an
// agreeing title (3), an agreeing title alone (2), then the position with
// only an agreeing length (1).
type trackEdge struct {
	query, track, tier, score int
	gap                       int64
}

func (e trackEdge) strategy() string {
	if e.tier == 2 {
		return strategyTrackTitle
	}
	return strategyTrackPosition
}

// MatchAlbumTracks pairs an album's pending local tracks with its Spotify
// track list. Every acceptable pairing is ranked and taken best first, so each
// Spotify track goes to one file: the one at its position with an agreeing
// title before one whose title merely agrees, and either before one at its
// position whose length alone agrees. A held track is already taken, and only
// a duplicate file, with the same title and length as the file holding a
// track, shares it. A file that loses every acceptable track to others misses
// with MatchReasonTrackClaimed; the results follow pending's order.
func MatchAlbumTracks(pending []TrackQuery, held []HeldTrack, tracks []SimpleTrack) []TrackPairing {
	owners := make(map[int]TrackQuery, len(held)+len(pending))
	for _, holder := range held {
		index := slices.IndexFunc(tracks, func(track SimpleTrack) bool { return track.ID == holder.ID })
		if index < 0 {
			continue
		}
		_, taken := owners[index]
		if !taken {
			owners[index] = holder.TrackQuery
		}
	}

	var edges []trackEdge
	for queryIndex, query := range pending {
		positioned := trackAtPosition(query, tracks)
		for trackIndex := range tracks {
			edge := trackEdge{query: queryIndex, track: trackIndex, score: scoreTrackTitle(query.Title, tracks[trackIndex].Name), gap: durationGap(query, tracks[trackIndex])}
			atPosition := positioned == &tracks[trackIndex]
			titleAgrees := edge.score >= spotifyTrackThreshold
			switch {
			case atPosition && titleAgrees:
				edge.tier = 3
			case titleAgrees:
				edge.tier = 2
			case atPosition && edge.gap <= trackDurationToleranceMs:
				edge.tier = 1
			default:
				continue
			}
			edges = append(edges, edge)
		}
	}
	// Stable, so a tie keeps the album's file order and Spotify's track order.
	slices.SortStableFunc(edges, func(a, b trackEdge) int {
		return cmp.Or(cmp.Compare(b.tier, a.tier), cmp.Compare(b.score, a.score), cmp.Compare(a.gap, b.gap))
	})

	results := make([]TrackPairing, len(pending))
	best := make([]*trackEdge, len(pending))
	for i := range edges {
		edge := &edges[i]
		if results[edge.query].Track != nil {
			continue
		}
		if best[edge.query] == nil {
			best[edge.query] = edge
		}
		query := pending[edge.query]
		owner, taken := owners[edge.track]
		if taken && !sameRecording(owner, query) {
			continue
		}
		if !taken {
			owners[edge.track] = query
		}
		results[edge.query] = TrackPairing{Track: &tracks[edge.track], Info: trackMatchInfo(query, tracks[edge.track], edge.score, edge.strategy(), MatchReasonAccepted)}
	}
	for i, query := range pending {
		if results[i].Track != nil {
			continue
		}
		if best[i] != nil {
			results[i].Info = trackMatchInfo(query, tracks[best[i].track], best[i].score, best[i].strategy(), MatchReasonTrackClaimed)
			continue
		}
		results[i].Info = nearestTrackTitle(query, tracks)
	}
	return results
}

// nearestTrackTitle explains a file no Spotify track accepts with the best
// title among them.
func nearestTrackTitle(query TrackQuery, tracks []SimpleTrack) MatchDebugInfo {
	if len(tracks) == 0 {
		return MatchDebugInfo{Lookup: lookupTrack, Input: query.Title, Strategy: strategyTrackPosition, Threshold: spotifyTrackThreshold, Reason: MatchReasonNoResults}
	}
	bestIndex, bestScore, bestGap := 0, -1, int64(0)
	for index := range tracks {
		score := scoreTrackTitle(query.Title, tracks[index].Name)
		gap := durationGap(query, tracks[index])
		better := score > bestScore || (score == bestScore && gap < bestGap)
		if !better {
			continue
		}
		bestIndex, bestScore, bestGap = index, score, gap
	}
	return trackMatchInfo(query, tracks[bestIndex], bestScore, strategyTrackTitle, MatchReasonScoreBelowThreshold)
}

func trackMatchInfo(query TrackQuery, track SimpleTrack, score int, strategy, reason string) MatchDebugInfo {
	return MatchDebugInfo{
		Lookup:        lookupTrack,
		Input:         query.Title,
		Strategy:      strategy,
		CandidateName: track.Name,
		Score:         score,
		Threshold:     spotifyTrackThreshold,
		Reason:        reason,
	}
}

// sameRecording reports a duplicate file: the same title and, within the
// length tolerance, the same length.
func sameRecording(a, b TrackQuery) bool {
	gap := a.DurationMs - b.DurationMs
	return normalizeComparisonText(a.Title) == normalizeComparisonText(b.Title) && max(gap, -gap) <= trackDurationToleranceMs
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
