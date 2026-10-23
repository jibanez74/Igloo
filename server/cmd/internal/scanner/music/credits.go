package music

import (
	"errors"
	"strings"

	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

type compoundArtistCredits struct {
	parts        []string
	occurrences  []string
	hasComma     bool
	hasDuplicate bool
}

func parseCompoundArtistCredits(artistTag string) compoundArtistCredits {
	commaParts := commaCredits(artistTag, false)

	credits := compoundArtistCredits{
		hasComma: strings.Contains(artistTag, ","),
	}
	seen := make(map[string]struct{}, len(commaParts))

	for _, commaPart := range commaParts {
		ampersandParts := strings.SplitSeq(commaPart, " & ")
		for rawPart := range ampersandParts {
			part := strings.TrimSpace(rawPart)
			if part == "" {
				continue
			}

			credits.occurrences = append(credits.occurrences, part)
			cacheKey := scanner.NormalizedScanCacheKey(part)
			_, exists := seen[cacheKey]
			if exists {
				credits.hasDuplicate = true
				continue
			}

			seen[cacheKey] = struct{}{}
			credits.parts = append(credits.parts, part)
		}
	}

	return credits
}

func shouldSplitCompoundArtistCreditsLocally(credits compoundArtistCredits) bool {
	if len(credits.parts) < 2 || !credits.hasComma {
		return false
	}

	if credits.hasDuplicate {
		return true
	}

	for _, part := range credits.parts {
		words := strings.Fields(part)
		if len(words) < 2 {
			return false
		}
	}

	return true
}

func shouldSplitCompoundArtistCredits(err error) bool {
	matchErr, ok := errors.AsType[*spotifyapi.MatchError](err)
	if !ok {
		return false
	}

	return musicSpotifyReasonSplitsCompound(matchErr.Info.Reason)
}

// commaCredits splits value on commas, attaching a name suffix to the credit
// before it, so "Leslie Odom, Jr." stays whole, also when " & " follows the
// suffix ("Leslie Odom, Jr. & Lin-Manuel Miranda"). keepEmpty retains empty
// positions, which sort credits need.
func commaCredits(value string, keepEmpty bool) []string {
	parts := []string{}
	for raw := range strings.SplitSeq(value, ",") {
		part := strings.TrimSpace(raw)
		if part == "" && !keepEmpty {
			continue
		}
		suffix, rest, _ := strings.Cut(part, " & ")
		suffix = strings.TrimSpace(suffix)
		if isArtistSuffix(suffix) && len(parts) > 0 {
			parts[len(parts)-1] += ", " + suffix
			part = strings.TrimSpace(rest)
			if part == "" {
				continue
			}
		}
		parts = append(parts, part)
	}
	return parts
}

func isArtistSuffix(value string) bool {
	suffix := strings.ToLower(strings.TrimSuffix(value, "."))

	switch suffix {
	case "jr", "sr", "ii", "iii", "iv":
		return true
	case "v", "vi":
		return strings.HasSuffix(value, ".")
	default:
		return false
	}
}

// parseArtistSortCredits retains positions and repeated values. Ampersands
// delimit inverted names without treating their internal commas as credits.
func parseArtistSortCredits(value string, count int) []string {
	parts := strings.Split(" "+value+" ", " & ")
	if len(parts) == count {
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}

	commaParts := commaCredits(value, true)
	parts = nil
	for _, part := range commaParts {
		for credit := range strings.SplitSeq(" "+part+" ", " & ") {
			parts = append(parts, strings.TrimSpace(credit))
		}
	}
	if len(parts) == count {
		return parts
	}
	return nil
}
