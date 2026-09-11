//go:build integration

package tmdb

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTVMetadataIntegration(t *testing.T) {
	apiKey := loadIntegrationEnv(t)
	var showID int
	for _, tc := range []struct {
		name string
		year []int
	}{
		{"title only", nil},
		{"premiere year", []int{2008}},
		{"unfiltered fallback", []int{1850}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh client makes every search (including fallback) hit TMDB.
			client, err := New(apiKey)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			shows, err := client.SearchShowsByTitleAndYear(ctx, "Breaking Bad", tc.year...)
			if err != nil {
				t.Fatal(err)
			}
			for _, show := range shows {
				name := strings.TrimSpace(show.Name)
				if show.ID <= 0 || name == "" {
					t.Fatalf("invalid search identity: %+v", show)
				}
				matches := show.Name == "Breaking Bad" && strings.HasPrefix(show.FirstAirDate, "2008-")
				if !matches {
					continue
				}
				if showID != 0 && show.ID != showID {
					t.Fatalf("search identity changed: got %d, want %d", show.ID, showID)
				}
				showID = show.ID
				t.Logf("resolved Breaking Bad (2008): show ID %d", showID)
				return
			}
			t.Fatal("search did not return Breaking Bad (2008)")
		})
	}
	if showID == 0 {
		t.Fatal("cannot check details without a search-resolved show ID")
	}
	client, err := New(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("show details", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		show, err := client.GetShowDetails(ctx, showID)
		if err != nil {
			t.Fatal(err)
		}
		validIdentity := show.ID == showID && show.Name == "Breaking Bad" && strings.HasPrefix(show.FirstAirDate, "2008-")
		if !validIdentity {
			t.Fatalf("unexpected show identity: %+v", show)
		}
		if show.OriginalName == "" || show.Overview == "" || show.OriginalLanguage == "" || show.NumberOfSeasons <= 0 || show.NumberOfEpisodes <= 0 {
			t.Error("missing show descriptions or episode/season totals")
		}
		certification := show.Certification()
		if show.ExternalIDs.IMDbID == "" || certification == "" || show.Videos.Results == nil {
			t.Error("missing appended external IDs, content ratings, or video results")
		}
		if len(show.Genres) == 0 || len(show.Networks) == 0 || len(show.ProductionCompanies) == 0 || len(show.CreatedBy) == 0 {
			t.Error("missing show relationships")
		}
		assertLiveTVAggregateCredits(t, show.AggregateCredits)
	})
	var pilot TVEpisode
	t.Run("season details", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		season, err := client.GetSeasonDetails(ctx, showID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if season.ID <= 0 || season.SeasonNumber != 1 || season.Name == "" || len(season.Episodes) == 0 {
			t.Fatalf("invalid season identity or episode list: %+v", season)
		}
		if season.Videos.Results == nil {
			t.Error("missing appended season video results")
		}
		assertLiveTVAggregateCredits(t, season.AggregateCredits)
		seenIDs := make(map[int]bool)
		seenNumbers := make(map[int]bool)
		for _, episode := range season.Episodes {
			if episode.ID <= 0 || seenIDs[episode.ID] || episode.SeasonNumber != 1 || episode.EpisodeNumber <= 0 || seenNumbers[episode.EpisodeNumber] || episode.Name == "" {
				t.Fatalf("invalid episode identity or numbering: %+v", episode)
			}
			seenIDs[episode.ID] = true
			seenNumbers[episode.EpisodeNumber] = true
			if episode.EpisodeNumber == 1 {
				pilot = episode
			}
		}
		if pilot.ID == 0 {
			t.Fatal("season 1 did not include episode 1")
		}
		t.Logf("resolved season ID %d, S01E01 ID %d", season.ID, pilot.ID)
	})
	t.Run("episode credits", func(t *testing.T) {
		if pilot.ID == 0 {
			t.Fatal("cannot compare credits without the season's episode 1 identity")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		credits, err := client.GetEpisodeCredits(ctx, showID, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if credits.ID != pilot.ID {
			t.Fatalf("credits ID %d differs from season episode ID %d", credits.ID, pilot.ID)
		}
		if len(credits.Cast) == 0 || credits.GuestStars == nil || len(credits.Crew) == 0 {
			t.Fatal("missing episode cast, guest stars, or crew")
		}
		for _, cast := range append(credits.Cast, credits.GuestStars...) {
			if cast.ID <= 0 || cast.Name == "" || cast.CreditID == "" {
				t.Errorf("invalid episode cast credit: %+v", cast)
			}
		}
		for _, crew := range credits.Crew {
			if crew.ID <= 0 || crew.Name == "" || crew.CreditID == "" || crew.Job == "" {
				t.Errorf("invalid episode crew credit: %+v", crew)
			}
		}
		t.Logf("episode credits identity matches S01E01: %d", credits.ID)
	})
}

func assertLiveTVAggregateCredits(t *testing.T, credits TVAggregateCredits) {
	t.Helper()
	if len(credits.Cast) == 0 || len(credits.Crew) == 0 {
		t.Fatal("missing aggregate cast or crew")
	}
	for _, cast := range credits.Cast {
		if cast.ID <= 0 || cast.Name == "" || len(cast.Roles) == 0 {
			t.Errorf("invalid aggregate cast identity or roles: %+v", cast)
		}
		for _, role := range cast.Roles {
			if role.CreditID == "" || role.EpisodeCount <= 0 {
				t.Errorf("invalid aggregate role: %+v", role)
			}
		}
	}
	for _, crew := range credits.Crew {
		if crew.ID <= 0 || crew.Name == "" || len(crew.Jobs) == 0 {
			t.Errorf("invalid aggregate crew identity or jobs: %+v", crew)
		}
		for _, job := range crew.Jobs {
			if job.CreditID == "" || job.Job == "" || job.EpisodeCount <= 0 {
				t.Errorf("invalid aggregate job: %+v", job)
			}
		}
	}
}
