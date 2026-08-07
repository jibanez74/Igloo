package helpers

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeComparisonText folds a display string into a stable comparison key:
// NFD decomposition with combining marks stripped (Beyoncé == Beyonce), & and +
// spelled as "and", punctuation collapsed to single spaces, and lowercased. It
// is both the entity identity key in the music schema (musicians.name_key,
// half of albums.album_key) and the normalization used by provider match
// scoring, so database identity and scoring identity can never disagree.
func NormalizeComparisonText(value string) string {
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

// NormalizeMBID lowercases and trims a MusicBrainz identifier and reports
// whether it has the canonical 8-4-4-4-12 UUID shape. Malformed MBIDs must be
// dropped at the tag boundary: Cover Art Archive answers them with 400 and
// MusicBrainz with 404, which would otherwise be recorded as spurious match
// failures.
func NormalizeMBID(value string) (string, bool) {
	mbid := strings.ToLower(strings.TrimSpace(value))
	if len(mbid) != 36 {
		return "", false
	}

	for i, r := range mbid {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
			if !isHex {
				return "", false
			}
		}
	}

	return mbid, true
}
