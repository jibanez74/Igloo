package musicbrainz

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const coverArtArchiveBaseURL = "https://coverartarchive.org/release-group"

var albumTitleSuffixes = regexp.MustCompile(`(?i)\s*(` +
	`\((?:` +
	`Deluxe(?:\s*(?:Version|Edition))?|` +
	`Bonus\s*Track\s*Version|` +
	`(?:(?:\d{4}|[A-Za-z]+)\s+)?Remaster(?:ed)?|` +
	`Special\s*Edition|` +
	`Expanded\s*Edition|` +
	`(?:\d+\w*\s+)?Anniversary\s*Edition|` +
	`Platinum\s*Edition|` +
	`Standard\s*Edition|` +
	`Explicit|` +
	`Clean\s*Version|` +
	`Live[^)]*|` +
	`Ao\s+Vivo[^)]*|` +
	`En\s+Directo[^)]*|` +
	`Edici[oó]n[^)]*|` +
	`Original\s+[^)]*(?:Soundtrack|Recording)|` +
	`Instrumental|` +
	`from\s+[^)]+|` +
	`[^)']*Version` +
	`)\)|` +
	`\[[^\]]*\]|` +
	`-\s*(?:Single|EP)|` +
	`\s+\+\s+.+` +
	`)\s*$`)

var featPattern = regexp.MustCompile(`(?i)\s*\((?:feat\.?|ft\.?|featuring)\s+[^)]+\)`)

func cleanAlbumTitle(title string) string {
	prev := title
	for {
		cleaned := strings.TrimSpace(albumTitleSuffixes.ReplaceAllString(prev, ""))
		if cleaned == prev || cleaned == "" {
			break
		}
		prev = cleaned
	}
	return prev
}

func stripFeaturing(title string) string {
	return strings.TrimSpace(featPattern.ReplaceAllString(title, ""))
}

func stripBeforeColon(title string) string {
	if idx := strings.LastIndex(title, ": "); idx > 0 {
		after := strings.TrimSpace(title[idx+2:])
		if after != "" {
			return after
		}
	}
	return ""
}

func stripAfterDash(title string) string {
	if idx := strings.Index(title, " - "); idx > 0 {
		before := strings.TrimSpace(title[:idx])
		if before != "" {
			return before
		}
	}
	return ""
}

func albumTitleVariants(title string) []string {
	seen := map[string]bool{title: true}
	var variants []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			variants = append(variants, v)
		}
	}

	cleaned := cleanAlbumTitle(title)
	add(cleaned)

	noFeat := stripFeaturing(title)
	add(noFeat)
	add(cleanAlbumTitle(noFeat))

	add(stripBeforeColon(title))
	add(stripBeforeColon(cleaned))

	add(stripAfterDash(title))
	add(stripAfterDash(cleaned))

	return variants
}

func (c *musicBrainzClient) SearchAlbumByName(title string, artistHint string) (*AlbumResult, error) {
	if title == "" {
		return nil, fmt.Errorf("album title cannot be empty")
	}
	cacheKey := title + "|" + artistHint
	if cached, exists := c.getAlbum(cacheKey); exists {
		return cached, nil
	}
	key := "album:" + cacheKey
	v, err, _ := c.sfGroup.Do(key, func() (interface{}, error) {
		if cached, exists := c.getAlbum(cacheKey); exists {
			return cached, nil
		}

		result, err := c.searchAlbum(title, artistHint)

		if err != nil {
			for _, variant := range albumTitleVariants(title) {
				result, err = c.searchAlbum(variant, artistHint)
				if err == nil {
					break
				}
			}
		}

		if err != nil && artistHint != "" {
			if primary := primaryArtist(artistHint); primary != "" {
				best := cleanAlbumTitle(stripFeaturing(title))
				result, err = c.searchAlbum(best, primary)
			}
		}

		if err != nil && artistHint != "" {
			for _, variant := range append([]string{cleanAlbumTitle(stripFeaturing(title))}, albumTitleVariants(title)...) {
				result, err = c.searchAlbum(variant, "")
				if err == nil {
					break
				}
			}
		}

		if err != nil {
			c.setAlbum(cacheKey, nil)
			return nil, err
		}
		c.setAlbum(cacheKey, result)
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*AlbumResult), nil
}

func (c *musicBrainzClient) searchAlbum(title, artistHint string) (*AlbumResult, error) {
	query := fmt.Sprintf(`releasegroup:"%s"`, title)
	if artistHint != "" {
		query += fmt.Sprintf(` AND artist:"%s"`, artistHint)
	}

	endpoint := fmt.Sprintf("%s/release-group?query=%s&fmt=json&limit=1", c.baseURL, url.QueryEscape(query))

	var rgResp releaseGroupSearchResponse
	if err := c.doRequest(endpoint, &rgResp); err != nil {
		return nil, fmt.Errorf("failed to search for album '%s': %w", title, err)
	}

	if len(rgResp.ReleaseGroups) == 0 {
		return nil, fmt.Errorf("no albums found for title '%s'", title)
	}

	rg := rgResp.ReleaseGroups[0]

	releaseID := pickReleaseID(rg.Releases)
	if releaseID == "" {
		return nil, fmt.Errorf("no releases found for album '%s'", title)
	}

	year := parseYear(rg.FirstReleaseDate)

	artistName := ""
	if len(rg.ArtistCredit) > 0 {
		artistName = rg.ArtistCredit[0].Name
	}

	return &AlbumResult{
		MusicBrainzID: rg.ID,
		ReleaseID:     releaseID,
		Title:         rg.Title,
		ReleaseDate:   rg.FirstReleaseDate,
		Year:          year,
		CoverURL:      fmt.Sprintf("%s/%s/front-500", coverArtArchiveBaseURL, rg.ID),
		ArtistName:    artistName,
	}, nil
}

func pickReleaseID(releases []releaseJSON) string {
	for _, r := range releases {
		if r.Status == "Official" {
			return r.ID
		}
	}
	if len(releases) > 0 {
		return releases[0].ID
	}
	return ""
}

func parseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}
