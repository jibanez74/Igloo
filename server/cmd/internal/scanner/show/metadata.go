package show

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
)

func applyShow(ctx context.Context, q *database.Queries, id int64, m *tmdb.TVShow) error {
	countries, err := json.Marshal(m.OriginCountry)
	if err != nil {
		return err
	}
	err = q.UpdateShowMetadata(ctx, database.UpdateShowMetadataParams{
		ID: id, Name: m.Name, TmdbID: helpers.NullInt64(int64(m.ID)), ImdbID: helpers.NullString(m.ExternalIDs.IMDbID), OriginalName: helpers.NullString(m.OriginalName), Overview: helpers.NullString(m.Overview), Tagline: helpers.NullString(m.Tagline), Language: helpers.NullString(m.OriginalLanguage), OriginCountries: helpers.NullString(string(countries)), FirstAirDate: helpers.NullString(m.FirstAirDate), LastAirDate: helpers.NullString(m.LastAirDate), Status: helpers.NullString(m.Status), Type: helpers.NullString(m.Type), Adult: m.Adult, PosterPath: helpers.NullString(m.PosterPath), BackdropPath: helpers.NullString(m.BackdropPath), Homepage: helpers.NullString(m.Homepage), VoteAverage: sql.NullFloat64{Float64: m.VoteAverage, Valid: true}, VoteCount: sql.NullInt64{Int64: int64(m.VoteCount), Valid: true}, Popularity: sql.NullFloat64{Float64: m.Popularity, Valid: true}, Certification: helpers.NullString(m.Certification()), TmdbSeasonCount: sql.NullInt64{Int64: int64(m.NumberOfSeasons), Valid: true}, TmdbEpisodeCount: sql.NullInt64{Int64: int64(m.NumberOfEpisodes), Valid: true},
	})
	if err != nil {
		return err
	}
	for _, remove := range []func(context.Context, int64) error{q.DeleteShowGenre, q.DeleteShowProductionCompany, q.DeleteShowNetwork, q.DeleteShowCreator} {
		err = remove(ctx, id)
		if err != nil {
			return err
		}
	}
	for _, g := range m.Genres {
		genreID, err := q.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{Tag: g.Name, GenreType: "show"})
		if err != nil {
			return err
		}
		err = q.CreateShowGenre(ctx, database.CreateShowGenreParams{ShowID: id, GenreID: genreID})
		if err != nil {
			return err
		}
	}
	for _, c := range m.ProductionCompanies {
		companyID, err := q.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{Name: c.Name, TmdbID: int64(c.ID)})
		if err != nil {
			return err
		}
		err = q.CreateShowProductionCompany(ctx, database.CreateShowProductionCompanyParams{ShowID: id, ProductionCompanyID: companyID})
		if err != nil {
			return err
		}
	}
	for _, n := range m.Networks {
		network, err := q.UpsertNetwork(ctx, database.UpsertNetworkParams{Name: n.Name, TmdbID: int64(n.ID), Logo: helpers.NullString(n.LogoPath), Country: helpers.NullString(n.OriginCountry)})
		if err != nil {
			return err
		}
		err = q.CreateShowNetwork(ctx, database.CreateShowNetworkParams{ShowID: id, NetworkID: network.ID})
		if err != nil {
			return err
		}
	}
	for _, c := range m.CreatedBy {
		artist, err := upsertPerson(ctx, q, c)
		if err != nil {
			return err
		}
		err = q.CreateShowCreator(ctx, database.CreateShowCreatorParams{ShowID: id, ArtistID: artist})
		if err != nil {
			return err
		}
	}
	err = replaceCredits(ctx, q, metadataOwner{show: id}, aggregateCredits(m.AggregateCredits))
	if err != nil {
		return err
	}
	err = replaceVideos(ctx, q, metadataOwner{show: id}, m.Videos.Results)
	if err != nil {
		return err
	}
	return q.ClearShowRetry(ctx, id)
}

func applySeason(ctx context.Context, q *database.Queries, id int64, m *tmdb.TVSeason) error {
	name := strings.TrimSpace(m.Name)
	if name == "" {
		name = fmt.Sprintf(seasonPlaceholderNameFormat, m.SeasonNumber)
	}
	err := q.UpdateShowSeasonMetadata(ctx, database.UpdateShowSeasonMetadataParams{ID: id, Name: name, TmdbID: helpers.NullInt64(int64(m.ID)), Overview: helpers.NullString(m.Overview), AirDate: helpers.NullString(m.AirDate), PosterPath: helpers.NullString(m.PosterPath), VoteAverage: sql.NullFloat64{Float64: m.VoteAverage, Valid: true}, TmdbEpisodeCount: sql.NullInt64{Int64: int64(len(m.Episodes)), Valid: true}})
	if err != nil {
		return err
	}
	err = replaceCredits(ctx, q, metadataOwner{season: id}, aggregateCredits(m.AggregateCredits))
	if err != nil {
		return err
	}
	err = replaceVideos(ctx, q, metadataOwner{season: id}, m.Videos.Results)
	if err != nil {
		return err
	}
	return q.ClearShowSeasonRetry(ctx, id)
}

func applyEpisode(ctx context.Context, q *database.Queries, id int64, m tmdb.TVEpisode) error {
	name := strings.TrimSpace(m.Name)
	if name == "" {
		name = fmt.Sprintf(episodePlaceholderNameFormat, m.EpisodeNumber)
	}
	err := q.UpdateShowEpisodeMetadata(ctx, database.UpdateShowEpisodeMetadataParams{ID: id, Name: name, TmdbID: helpers.NullInt64(int64(m.ID)), Overview: helpers.NullString(m.Overview), AirDate: helpers.NullString(m.AirDate), StillPath: helpers.NullString(m.StillPath), ProductionCode: helpers.NullString(m.ProductionCode), TmdbRuntime: helpers.NullInt64(int64(m.Runtime)), VoteAverage: sql.NullFloat64{Float64: m.VoteAverage, Valid: true}, VoteCount: sql.NullInt64{Int64: int64(m.VoteCount), Valid: true}})
	if err != nil {
		return err
	}
	rows := make([]credit, 0, len(m.GuestStars)+len(m.Crew))
	for _, c := range m.GuestStars {
		rows = append(rows, credit{person: c.TVPerson, character: c.Character, order: c.Order, creditID: c.CreditID, count: 1, guest: true})
	}
	for _, c := range m.Crew {
		rows = append(rows, credit{person: c.TVPerson, department: c.Department, job: c.Job, creditID: c.CreditID, count: 1, crew: true})
	}
	err = replaceCredits(ctx, q, metadataOwner{episode: id}, rows)
	if err != nil {
		return err
	}
	return q.ClearShowEpisodeRetry(ctx, id)
}

type metadataOwner struct{ show, season, episode int64 }
type credit struct {
	person                               tmdb.TVPerson
	character, department, job, creditID string
	order, count                         int
	crew, guest                          bool
}

func aggregateCredits(c tmdb.TVAggregateCredits) []credit {
	rows := make([]credit, 0)
	for _, p := range c.Cast {
		for _, r := range p.Roles {
			rows = append(rows, credit{person: p.TVPerson, character: r.Character, order: p.Order, creditID: r.CreditID, count: r.EpisodeCount})
		}
	}
	for _, p := range c.Crew {
		for _, r := range p.Jobs {
			rows = append(rows, credit{person: p.TVPerson, department: p.Department, job: r.Job, creditID: r.CreditID, count: r.EpisodeCount, crew: true})
		}
	}
	return rows
}
func upsertPerson(ctx context.Context, q *database.Queries, p tmdb.TVPerson) (int64, error) {
	if p.ID <= 0 {
		return 0, errors.New("invalid TMDB person identity")
	}
	return q.UpsertArtist(ctx, database.UpsertArtistParams{Name: p.Name, TmdbID: int64(p.ID), Profile: helpers.NullString(p.ProfilePath)})
}
func replaceCredits(ctx context.Context, q *database.Queries, owner metadataOwner, rows []credit) error {
	var remove []func(context.Context, int64) error
	var id int64
	switch {
	case owner.show != 0:
		id = owner.show
		remove = []func(context.Context, int64) error{q.DeleteShowCast, q.DeleteShowCrew}
	case owner.season != 0:
		id = owner.season
		remove = []func(context.Context, int64) error{q.DeleteShowSeasonCast, q.DeleteShowSeasonCrew}
	default:
		id = owner.episode
		remove = []func(context.Context, int64) error{q.DeleteShowEpisodeGuestCast, q.DeleteShowEpisodeCrew}
	}
	for _, fn := range remove {
		err := fn(ctx, id)
		if err != nil {
			return err
		}
	}
	people := make(map[int]int64)
	for _, c := range rows {
		artist, found := people[c.person.ID]
		var err error
		if !found {
			artist, err = upsertPerson(ctx, q, c.person)
			if err != nil {
				return err
			}
			people[c.person.ID] = artist
		}
		switch {
		case owner.show != 0:
			if c.crew {
				err = q.CreateShowCrew(ctx, database.CreateShowCrewParams{ShowID: id, ArtistID: artist, Department: c.department, Job: c.job, CreditID: c.creditID, EpisodeCount: int64(c.count)})
			} else {
				err = q.CreateShowCast(ctx, database.CreateShowCastParams{ShowID: id, ArtistID: artist, Character: c.character, CastOrder: int64(c.order), CreditID: c.creditID, EpisodeCount: int64(c.count)})
			}
		case owner.season != 0:
			if c.crew {
				err = q.CreateShowSeasonCrew(ctx, database.CreateShowSeasonCrewParams{SeasonID: id, ArtistID: artist, Department: c.department, Job: c.job, CreditID: c.creditID, EpisodeCount: int64(c.count)})
			} else {
				err = q.CreateShowSeasonCast(ctx, database.CreateShowSeasonCastParams{SeasonID: id, ArtistID: artist, Character: c.character, CastOrder: int64(c.order), CreditID: c.creditID, EpisodeCount: int64(c.count)})
			}
		default:
			if c.crew {
				err = q.CreateShowEpisodeCrew(ctx, database.CreateShowEpisodeCrewParams{EpisodeID: id, ArtistID: artist, Department: c.department, Job: c.job, CreditID: c.creditID, EpisodeCount: int64(c.count)})
			} else {
				err = q.CreateShowEpisodeGuestCast(ctx, database.CreateShowEpisodeGuestCastParams{EpisodeID: id, ArtistID: artist, Character: c.character, CastOrder: int64(c.order), CreditID: c.creditID, EpisodeCount: int64(c.count)})
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func replaceVideos(ctx context.Context, q *database.Queries, owner metadataOwner, videos []tmdb.TmdbVideoResult) error {
	var err error
	if owner.show != 0 {
		err = q.DeleteShowExtraVideo(ctx, owner.show)
	} else {
		err = q.DeleteShowSeasonExtraVideo(ctx, owner.season)
	}
	if err != nil {
		return err
	}
	for _, v := range videos {
		if v.Key == "" || v.ID == "" {
			continue
		}
		title := strings.TrimSpace(v.Name)
		if title == "" {
			title = v.Key
		}
		extraID, err := q.UpsertExtraVideo(ctx, database.UpsertExtraVideoParams{Title: title, ExternalID: helpers.NullString(v.ID), Key: v.Key, Type: v.ExtraVideoType(), Site: v.ExtraVideoSite()})
		if err != nil {
			return err
		}
		if owner.show != 0 {
			err = q.CreateShowExtraVideo(ctx, database.CreateShowExtraVideoParams{ShowID: owner.show, ExtraVideoID: extraID})
		} else {
			err = q.CreateShowSeasonExtraVideo(ctx, database.CreateShowSeasonExtraVideoParams{SeasonID: owner.season, ExtraVideoID: extraID})
		}
		if err != nil {
			return err
		}
	}
	return nil
}
