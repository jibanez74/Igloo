package musicbrainz

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var artistSeparators = regexp.MustCompile(`(?i)\s*(?:,\s*|\s+&\s+|\s+and\s+|\s+feat\.?\s+|\s+featuring\s+|\s+ft\.?\s+|\s+vs\.?\s+|\s+with\s+)`)

func primaryArtist(name string) string {
	loc := artistSeparators.FindStringIndex(name)
	if loc == nil {
		return ""
	}
	primary := strings.TrimSpace(name[:loc[0]])
	if primary == "" || primary == name {
		return ""
	}
	return primary
}

func (c *musicBrainzClient) SearchArtistByName(name string) (*ArtistResult, error) {
	if name == "" {
		return nil, fmt.Errorf("artist name cannot be empty")
	}
	if cached, exists := c.getArtist(name); exists {
		return cached, nil
	}
	key := "artist:" + name
	v, err, _ := c.sfGroup.Do(key, func() (interface{}, error) {
		if cached, exists := c.getArtist(name); exists {
			return cached, nil
		}
		result, err := c.searchArtist(name)
		if err != nil {
			if primary := primaryArtist(name); primary != "" {
				result, err = c.searchArtist(primary)
			}
		}
		if err != nil {
			c.setArtist(name, nil)
			return nil, err
		}
		c.setArtist(name, result)
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*ArtistResult), nil
}

func (c *musicBrainzClient) searchArtist(name string) (*ArtistResult, error) {
	query := fmt.Sprintf(`artist:"%s"`, name)
	endpoint := fmt.Sprintf("%s/artist?query=%s&fmt=json&limit=1", c.baseURL, url.QueryEscape(query))

	var resp artistSearchResponse
	if err := c.doRequest(endpoint, &resp); err != nil {
		return nil, fmt.Errorf("failed to search for artist '%s': %w", name, err)
	}

	if len(resp.Artists) == 0 {
		return nil, fmt.Errorf("no artists found for name '%s'", name)
	}

	a := resp.Artists[0]
	return &ArtistResult{
		MusicBrainzID:  a.ID,
		Name:           a.Name,
		SortName:       a.SortName,
		Country:        a.Country,
		Type:           a.Type,
		Disambiguation: a.Disambiguation,
	}, nil
}
