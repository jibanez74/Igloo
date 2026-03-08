package musicbrainz

import "fmt"

const audioDBBaseURL = "https://www.theaudiodb.com/api/v1/json"

type audioDBResponse struct {
	Artists []audioDBArtist `json:"artists"`
}

type audioDBArtist struct {
	ArtistThumb string `json:"strArtistThumb"`
}

type audioDBAlbumResponse struct {
	Albums []audioDBAlbum `json:"album"`
}

type audioDBAlbum struct {
	AlbumThumb string `json:"strAlbumThumb"`
}

func (c *musicBrainzClient) GetArtistImageURL(musicBrainzID string) (string, error) {
	if musicBrainzID == "" {
		return "", fmt.Errorf("musicbrainz ID cannot be empty")
	}

	if cached, exists := c.getImage(musicBrainzID); exists {
		if cached == "" {
			return "", fmt.Errorf("no artist image found for '%s' (cached)", musicBrainzID)
		}
		return cached, nil
	}
	key := "image:" + musicBrainzID
	v, err, _ := c.sfGroup.Do(key, func() (interface{}, error) {
		if cached, exists := c.getImage(musicBrainzID); exists {
			if cached == "" {
				return "", fmt.Errorf("no artist image found for '%s' (cached)", musicBrainzID)
			}
			return cached, nil
		}
		endpoint := fmt.Sprintf("%s/%s/artist-mb.php?i=%s", c.audioDBBaseURL, c.audioDBAPIKey, musicBrainzID)
		var resp audioDBResponse
		if err := c.doAudioDBRequest(endpoint, &resp); err != nil {
			c.setImage(musicBrainzID, "")
			return "", fmt.Errorf("failed to lookup artist image for '%s': %w", musicBrainzID, err)
		}
		if len(resp.Artists) == 0 || resp.Artists[0].ArtistThumb == "" {
			c.setImage(musicBrainzID, "")
			return "", fmt.Errorf("no artist image found for '%s'", musicBrainzID)
		}
		imageURL := resp.Artists[0].ArtistThumb
		c.setImage(musicBrainzID, imageURL)
		return imageURL, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (c *musicBrainzClient) GetAlbumImageURL(releaseGroupID string) (string, error) {
	if releaseGroupID == "" {
		return "", fmt.Errorf("release group ID cannot be empty")
	}

	cacheKey := "albumart:" + releaseGroupID
	if cached, exists := c.getImage(cacheKey); exists {
		if cached == "" {
			return "", fmt.Errorf("no album image found for '%s' (cached)", releaseGroupID)
		}
		return cached, nil
	}

	sfKey := "albumimage:" + releaseGroupID
	v, err, _ := c.sfGroup.Do(sfKey, func() (interface{}, error) {
		if cached, exists := c.getImage(cacheKey); exists {
			if cached == "" {
				return "", fmt.Errorf("no album image found for '%s' (cached)", releaseGroupID)
			}
			return cached, nil
		}
		endpoint := fmt.Sprintf("%s/%s/album-mb.php?i=%s", c.audioDBBaseURL, c.audioDBAPIKey, releaseGroupID)
		var resp audioDBAlbumResponse
		if err := c.doAudioDBRequest(endpoint, &resp); err != nil {
			c.setImage(cacheKey, "")
			return "", fmt.Errorf("failed to lookup album image for '%s': %w", releaseGroupID, err)
		}
		if len(resp.Albums) == 0 || resp.Albums[0].AlbumThumb == "" {
			c.setImage(cacheKey, "")
			return "", fmt.Errorf("no album image found for '%s'", releaseGroupID)
		}
		imageURL := resp.Albums[0].AlbumThumb
		c.setImage(cacheKey, imageURL)
		return imageURL, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}
