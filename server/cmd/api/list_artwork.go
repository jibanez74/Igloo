package main

import (
	"database/sql"
	"encoding/json"
	"slices"
)

// maxPlaylistArtwork is how many images a playlist card's collage shows.
const maxPlaylistArtwork = 4

// decodeImageList reads the JSON array of image references a list query built
// with json_group_array. The result is never nil, so a response carries [].
func decodeImageList(raw string) ([]string, error) {
	images := []string{}

	err := json.Unmarshal([]byte(raw), &images)
	if err != nil {
		return nil, err
	}

	return images, nil
}

// pickGenreBackdrops gives each genre its first candidate that no other genre
// took, so the tiles show different titles. Genres with the fewest candidates
// choose first: a genre with one movie has nothing else to show, while a large
// genre can step past a taken image. A genre whose candidates are all taken
// repeats its first one; a genre with none gets no backdrop. The result is in
// the order of candidates.
func pickGenreBackdrops(candidates [][]string) []sql.NullString {
	picked := make([]sql.NullString, len(candidates))
	taken := make(map[string]struct{}, len(candidates))

	scarcestFirst := make([]int, len(candidates))
	for i := range candidates {
		scarcestFirst[i] = i
	}
	slices.SortStableFunc(scarcestFirst, func(a, b int) int {
		return len(candidates[a]) - len(candidates[b])
	})

	for _, i := range scarcestFirst {
		images := candidates[i]
		if len(images) == 0 {
			continue
		}

		choice := images[0]
		for _, image := range images {
			_, isTaken := taken[image]
			if !isTaken {
				choice = image
				break
			}
		}

		taken[choice] = struct{}{}
		picked[i] = sql.NullString{String: choice, Valid: true}
	}

	return picked
}

// distinctImages keeps the first limit different images, in order: a playlist
// that opens with several tracks of one album shows that cover once.
func distinctImages(images []string, limit int) []string {
	distinct := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)

	for _, image := range images {
		isFull := len(distinct) == limit
		if isFull {
			break
		}

		_, isSeen := seen[image]
		if isSeen {
			continue
		}

		seen[image] = struct{}{}
		distinct = append(distinct, image)
	}

	return distinct
}
