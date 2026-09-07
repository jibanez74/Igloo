package music

import (
	"strings"

	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

type compoundArtistCredits struct {
	parts        []string
	hasDelimiter bool
	hasComma     bool
	hasDuplicate bool
}

func parseCompoundArtistCredits(artistTag string) compoundArtistCredits {
	rawCommaParts := strings.Split(artistTag, ",")
	commaParts := make([]string, 0, len(rawCommaParts))

	for _, rawPart := range rawCommaParts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			continue
		}

		isSuffix := isArtistSuffix(part)
		if isSuffix && len(commaParts) > 0 {
			lastIndex := len(commaParts) - 1
			commaParts[lastIndex] = commaParts[lastIndex] + ", " + part
			continue
		}

		commaParts = append(commaParts, part)
	}

	credits := compoundArtistCredits{
		hasDelimiter: strings.Contains(artistTag, " & ") || strings.Contains(artistTag, ","),
		hasComma:     strings.Contains(artistTag, ","),
	}
	seen := make(map[string]struct{}, len(commaParts))

	for _, commaPart := range commaParts {
		ampersandParts := strings.Split(commaPart, " & ")
		for _, rawPart := range ampersandParts {
			part := strings.TrimSpace(rawPart)
			if part == "" {
				continue
			}

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
	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		return false
	}

	return musicSpotifyReasonSplitsCompound(matchErr.Info.Reason)
}

func isArtistSuffix(value string) bool {
	suffix := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))

	switch suffix {
	case "jr", "sr", "ii", "iii", "iv", "v", "vi":
		return true
	default:
		return false
	}
}
