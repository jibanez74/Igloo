package show

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var seasonDirectory = regexp.MustCompile(`(?i)^(?:season\s+(\d{1,4})(?:\s+-\s+\S.*)?|s(\d{1,4}))$`)
var episodeToken = regexp.MustCompile(`(?i)s(\d{1,4})e(\d{1,4})((?:-e\d{1,4}|e\d{1,4})*)|(\d{1,4})x(\d{1,4})`)
var conflictingEpisode = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])e\d+`)
var episodeSuffix = regexp.MustCompile(`(?i)(-?)e(\d{1,4})`)
var episodeTitleSeparator = regexp.MustCompile(`^\s+-\s+`)
var explicitYear = regexp.MustCompile(`^(.*?)\s*\(\s*((?:19|20)\d{2})\s*\)\s*$`)
var titleYear = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)

type localEpisodeFile struct {
	showPath string
	title    string
	year     int
	season   int
	episodes []int
}

// parseFile accepts Show/Season N/episode and Show/episode layouts. With a
// season directory the token must agree with it; without one the token alone
// names the season.
func parseFile(root, path string) (localEpisodeFile, error) {
	var result localEpisodeFile
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return result, err
	}
	parts := strings.Split(relative, string(filepath.Separator))
	if len(parts) != 2 && len(parts) != 3 {
		return result, fmt.Errorf("expected show/[season/]episode path")
	}
	for _, part := range parts {
		hidden := strings.HasPrefix(part, ".")
		if hidden {
			return result, fmt.Errorf("hidden entry")
		}
	}
	result.showPath = filepath.Join(root, parts[0])
	result.title, result.year, _ = parseShowTitle(parts[0])
	hasSeasonDir := len(parts) == 3
	if hasSeasonDir {
		directorySeason, ok := parseSeasonDirectory(parts[1])
		if !ok {
			return result, fmt.Errorf("invalid season directory")
		}
		result.season = directorySeason
	}
	name := parts[len(parts)-1]
	base := strings.TrimSuffix(name, filepath.Ext(name))
	matches := episodeToken.FindAllStringSubmatchIndex(base, -1)
	episodeMatches := matches[:0]
	for _, m := range matches {
		// Standalone 3–4-digit dimension pairs are resolution metadata.
		isResolution := m[8] >= 0 && m[9]-m[8] >= 3 && m[11]-m[10] >= 3
		validPrefix := m[0] == 0 || !isASCIIAlphaNumeric(base[m[0]-1])
		validSuffix := m[1] == len(base) || !isASCIIAlphaNumeric(base[m[1]])
		if isResolution && validPrefix && validSuffix {
			continue
		}
		episodeMatches = append(episodeMatches, m)
	}
	matches = episodeMatches
	if len(matches) != 1 {
		return result, fmt.Errorf("missing or conflicting episode numbering")
	}
	m := matches[0]
	// Reject partial token matches such as S01E01-E, S01E01-03 or S01E01E99999.
	invalidPrefix := m[0] > 0 && isASCIIAlphaNumeric(base[m[0]-1])
	if invalidPrefix {
		return result, fmt.Errorf("invalid episode token boundary")
	}
	tail := base[m[1]:]
	conflicting := conflictingEpisode.MatchString(base[:m[0]]) || conflictingEpisode.MatchString(tail)
	if conflicting {
		return result, fmt.Errorf("conflicting episode numbering")
	}
	invalidSuffix := len(tail) > 0 && isASCIIAlphaNumeric(tail[0])
	if invalidSuffix {
		return result, fmt.Errorf("invalid episode token suffix")
	}
	titleSeparator := episodeTitleSeparator.MatchString(tail)
	trimmed := strings.TrimLeft(tail, " ._\t\r\n\f")
	rangeSuffix := strings.HasPrefix(trimmed, "-")
	if rangeSuffix && !titleSeparator {
		after := strings.TrimLeft(trimmed[1:], " ._\t\r\n\f")
		looksNumbered := len(after) > 0 && (after[0] >= '0' && after[0] <= '9' || (after[0] == 'E' || after[0] == 'e') && (len(after) == 1 || !isASCIIAlphaNumeric(after[1]) || after[1] >= '0' && after[1] <= '9'))
		if looksNumbered {
			return result, fmt.Errorf("malformed episode range")
		}
	}
	var season, first int
	if m[2] >= 0 {
		season, _ = strconv.Atoi(base[m[2]:m[3]])
		first, _ = strconv.Atoi(base[m[4]:m[5]])
		result.episodes = []int{first}
		seen := map[int]bool{first: true}
		suffix := base[m[6]:m[7]]
		for _, token := range episodeSuffix.FindAllStringSubmatch(suffix, -1) {
			number, _ := strconv.Atoi(token[2])
			start := number
			if token[1] == "-" {
				last := result.episodes[len(result.episodes)-1]
				if number <= last {
					return result, fmt.Errorf("reversed or empty episode range")
				}
				start = last + 1
			}
			for ep := start; ep <= number; ep++ {
				if ep <= 0 || seen[ep] {
					return result, fmt.Errorf("duplicate episode numbering")
				}
				seen[ep] = true
				result.episodes = append(result.episodes, ep)
			}
		}
	} else {
		season, _ = strconv.Atoi(base[m[8]:m[9]])
		first, _ = strconv.Atoi(base[m[10]:m[11]])
		result.episodes = []int{first}
	}
	if first <= 0 {
		return result, fmt.Errorf("invalid episode number")
	}
	if hasSeasonDir && season != result.season {
		return result, fmt.Errorf("episode numbering disagrees with season directory")
	}
	result.season = season
	return result, nil
}

// parseSeasonDirectory accepts Specials (season 0), "Season N", "Season N -
// Title", and "SN".
func parseSeasonDirectory(name string) (int, bool) {
	if strings.EqualFold(name, "Specials") {
		return 0, true
	}
	match := seasonDirectory.FindStringSubmatch(name)
	if match == nil {
		return 0, false
	}
	digits := match[1]
	if digits == "" {
		digits = match[2]
	}
	season, _ := strconv.Atoi(digits)
	return season, true
}

func isASCIIAlphaNumeric(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func parseShowTitle(folder string) (string, int, string) {
	name := strings.TrimSpace(strings.NewReplacer(".", " ", "_", " ").Replace(folder))
	match := explicitYear.FindStringSubmatch(name)
	hasExplicitYear := match != nil && strings.TrimSpace(match[1]) != ""
	if hasExplicitYear {
		year, _ := strconv.Atoi(match[2])
		return strings.TrimSpace(match[1]), year, ""
	}
	matches := titleYear.FindAllStringIndex(name, -1)
	if len(matches) == 1 {
		m := matches[0]
		parsed := strings.TrimSpace(name[:m[0]] + " " + name[m[1]:])
		if parsed != "" {
			year, _ := strconv.Atoi(name[m[0]:m[1]])
			return parsed, year, name
		}
	}
	return name, 0, ""
}

func allowDirectory(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	parts := strings.Split(relative, string(filepath.Separator))
	if len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		hidden := strings.HasPrefix(part, ".")
		if hidden {
			return false
		}
	}
	if len(parts) == 1 {
		return true
	}
	_, ok := parseSeasonDirectory(parts[1])
	return ok
}
