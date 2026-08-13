package main

import (
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *Application) InitRouter() {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(preserveClientSocketIP)
	router.Use(middleware.RealIP)
	router.Use(app.RequestLogger)
	router.Use(middleware.Recoverer)

	app.registerWebSocketRoutes(router)

	router.Group(func(r chi.Router) {
		r.Use(app.LoadAndSaveSession)
		// The session middleware is where io.ReaderFrom is lost, so the
		// capability is restored immediately inside it.
		r.Use(restoreSendfile)
		app.registerSessionRoutes(r)
	})

	app.Router = router
}

func (app *Application) registerWebSocketRoutes(router chi.Router) {
	router.With(app.DeviceTokenAuth, app.LoadSessionReadOnly, app.IsAuth).Get("/api/watch-rooms/{id}/ws", app.WatchRoomWebSocket)
}

func (app *Application) registerSessionRoutes(r chi.Router) {
	app.registerAPIRoutes(r)

	// Register SPA fallback after /api routes so API paths cannot be captured.
	// HEAD needs its own registration: chi resolves methods separately, so a
	// GET-only route answers HEAD with 405. ServeFrontend serves HEAD correctly
	// through http.ServeContent.
	r.Get("/*", app.ServeFrontend)
	r.Head("/*", app.ServeFrontend)
}

func (app *Application) registerAPIRoutes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", app.HealthCheck)
		r.Post("/auth/login", app.AuthenticateUser)
		r.Post("/auth/device-login", app.AuthenticateDevice)
		r.With(app.DeviceTokenAuth).Get("/auth/user", app.GetCurrentAuthUser)
		r.Post("/quick-connect/initiate", app.InitiateQuickConnect)
		r.Post("/quick-connect/redeem", app.RedeemQuickConnect)
		app.registerAuthenticatedAPIRoutes(r)
	})
}

func (app *Application) registerAuthenticatedAPIRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(app.DeviceTokenAuth)
		r.Use(app.IsAuth)

		app.registerAuthRoutes(r)
		app.registerDeviceRoutes(r)
		app.registerUserRoutes(r)
		app.registerNotificationRoutes(r)
		r.Get("/static/*", app.ServeStaticFiles)
		app.registerTMDBRoutes(r)
		app.registerYouTubeRoutes(r)
		app.registerSpotifyRoutes(r)
		app.registerSearchRoutes(r)
		app.registerMovieRoutes(r)
		r.Get("/users", app.GetUsers)
		app.registerAdminUserRoutes(r)
		app.registerWatchRoomRoutes(r)
		app.registerSettingsRoutes(r)
		app.registerMusicRoutes(r)
		app.registerPprof(r)
	})
}

func (app *Application) registerAuthRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Delete("/logout", app.DestroySession)
	})
}

func (app *Application) registerDeviceRoutes(r chi.Router) {
	r.Post("/quick-connect/lookup", app.LookupQuickConnect)
	r.Post("/quick-connect/approve", app.ApproveQuickConnect)

	r.Route("/devices", func(r chi.Router) {
		r.Get("/", app.GetDevices)
		r.Patch("/{id}", app.RenameDevice)
		r.Delete("/{id}", app.RevokeDevice)
	})
}

func (app *Application) registerUserRoutes(r chi.Router) {
	r.Route("/user", func(r chi.Router) {
		r.Put("/name", app.UpdateUserName)
		r.Put("/email", app.UpdateUserEmail)
		r.Put("/password", app.UpdateUserPassword)
		r.Get("/pin", app.GetUserPin)
		r.Put("/pin", app.UpdateUserPin)
		r.Post("/pin/verify", app.VerifyUserPin)
		r.Put("/avatar", app.UpdateUserAvatar)
		r.Post("/avatar/upload", app.UploadUserAvatar)
		r.Delete("/", app.DeleteUserAccount)
	})
}

func (app *Application) registerNotificationRoutes(r chi.Router) {
	r.Route("/notifications", func(r chi.Router) {
		r.Get("/", app.ListNotifications)
		r.Post("/", app.CreateNotification)
		r.Get("/unread-count", app.GetUnreadNotificationCount)
		r.Post("/read-all", app.MarkAllNotificationsRead)
		r.Post("/{id}/read", app.MarkNotificationRead)
		r.Delete("/{id}", app.DeleteNotification)
	})
}

func (app *Application) registerTMDBRoutes(r chi.Router) {
	r.Route("/tmdb", func(r chi.Router) {
		r.Get("/status", app.GetTmdbStatus)
		r.Get("/images/{size}/{file}", app.ProxyTmdbImage)
		r.Post("/movies/search", app.SearchTmdbMovies)
		r.Get("/movies/in-theaters", app.GetMoviesInTheaters)
		r.Get("/movies/{id}", app.GetMovieByTmdbID)
	})
}

func (app *Application) registerYouTubeRoutes(r chi.Router) {
	r.Route("/youtube", func(r chi.Router) {
		r.Get("/thumbnails/{key}", app.ProxyYouTubeThumbnail)
	})
}

func (app *Application) registerSpotifyRoutes(r chi.Router) {
	r.Route("/spotify", func(r chi.Router) {
		r.Get("/status", app.GetSpotifyStatus)
		r.Post("/albums/search", app.SearchSpotifyAlbums)
		r.Post("/tracks/search", app.SearchSpotifyTracks)
	})
}

func (app *Application) registerSearchRoutes(r chi.Router) {
	r.Get("/search", app.SearchAll)
	r.Route("/search", func(r chi.Router) {
		r.Get("/", app.SearchAll)
		r.Get("/movies", app.SearchMovies)
		r.Get("/albums", app.SearchAlbums)
		r.Get("/musicians", app.SearchMusicians)
		r.Get("/tracks", app.SearchTracks)
	})
}

func (app *Application) registerMovieRoutes(r chi.Router) {
	r.Route("/movies", func(r chi.Router) {
		r.Get("/latest", app.GetLatestMovies)
		r.Get("/continue-watching", app.GetContinueWatchingMovies)
		r.Get("/library", app.GetMoviesLibrary)
		r.Get("/stats", app.GetMoviesStats)
		r.Get("/liked", app.GetLikedMovies)
		r.Get("/{id}/like-status", app.GetMovieLikeStatus)
		r.Get("/genres", app.GetMovieGenresList)
		r.Get("/genres/{genreId}/movies", app.GetMoviesByGenreLibrary)
		app.registerMoviePlaylistRoutes(r)
		r.Get("/details/{id}", app.GetMovieDetails)
		r.Get("/{id}/technical-details", app.GetMovieTechnicalDetails)
		r.Get("/{id}/watch-progress", app.GetMovieWatchProgress)
		r.Post("/{id}/like", app.ToggleLikeMovie)
		r.Put("/{id}/watch-progress", app.UpdateMovieWatchProgress)
		r.Delete("/{id}/watch-progress", app.DeleteMovieWatchProgress)
		r.Put("/{id}/watch-progress/watched", app.SetMovieWatched)
		r.Post("/{id}/hls/session/stop", app.StopPersonalHLSSession)
		r.Get("/{id}/hls/{profile}/"+helpers.HLS_PLAYLIST_FILENAME, app.HLSManifest)
		r.Get("/{id}/hls/{profile}/{filename}", app.HLSSegment)
		r.Get("/{id}/stream", app.StreamMovie)
		r.Head("/{id}/stream", app.StreamMovie)
		r.Get("/{id}/subtitles/{trackIndex}/web.vtt", app.SubtitleWebVTT)

		r.With(app.RequireAdmin).Post("/{id}/tmdb-search", app.TmdbSearchMovies)
		r.With(app.RequireAdmin).Put("/{id}/identify", app.IdentifyMovie)
		r.With(app.RequireAdmin).Patch("/{id}", app.UpdateMovieMetadata)
		r.With(app.RequireAdmin).Delete("/{id}", app.DeleteMovie)
	})
}

func (app *Application) registerMoviePlaylistRoutes(r chi.Router) {
	r.Route("/playlists", func(pr chi.Router) {
		pr.Get("/{id}/movies", app.GetMoviePlaylistMovies)
		pr.Post("/{id}/movies", app.AddMoviesToMoviePlaylist)
		pr.Delete("/{id}/movies/{movieId}", app.RemoveMovieFromMoviePlaylist)
		pr.Get("/{id}/collaborators", app.GetMoviePlaylistCollaborators)
		pr.Post("/{id}/collaborators", app.AddMoviePlaylistCollaborator)
		pr.Delete("/{id}/collaborators/{userId}", app.RemoveMoviePlaylistCollaborator)
		pr.Get("/", app.GetMoviePlaylists)
		pr.Post("/", app.CreateMoviePlaylist)
		pr.Get("/{id}", app.GetMoviePlaylist)
		pr.Put("/{id}", app.UpdateMoviePlaylist)
		pr.Delete("/{id}", app.DeleteMoviePlaylist)
	})
}

func (app *Application) registerAdminUserRoutes(r chi.Router) {
	r.Route("/admin/users", func(r chi.Router) {
		r.Use(app.RequireAdmin)
		r.Get("/", app.AdminGetUsers)
		r.Post("/", app.AdminCreateUser)
		r.Patch("/{id}", app.AdminUpdateUser)
		r.Delete("/{id}", app.AdminDeleteUser)
		r.Put("/{id}/password", app.AdminResetUserPassword)
	})
}

func (app *Application) registerWatchRoomRoutes(r chi.Router) {
	r.Route("/watch-rooms", func(r chi.Router) {
		r.Get("/", app.GetWatchRooms)
		r.Post("/", app.CreateWatchRoom)
		r.Get("/{id}", app.GetWatchRoom)
		r.Post("/{id}/join", app.JoinWatchRoom)
		r.Get("/{id}/stream", app.StreamWatchRoomMovie)
		r.Head("/{id}/stream", app.StreamWatchRoomMovie)
		r.Get("/{id}/hls/"+helpers.HLS_PLAYLIST_FILENAME, app.WatchRoomHLSManifest)
		r.Get("/{id}/hls/{filename}", app.WatchRoomHLSSegment)
		r.Delete("/{id}", app.DeleteWatchRoom)
	})
}

func (app *Application) registerSettingsRoutes(r chi.Router) {
	r.Route("/settings", func(r chi.Router) {
		r.Get("/", app.GetSettings)
		r.With(app.RequireAdmin).Get("/general", app.GetGeneralSettings)
		r.With(app.RequireAdmin).Put("/general", app.UpdateGeneralSettings)
		r.With(app.RequireAdmin).Put("/libraries", app.UpdateLibrarySettings)
		r.Get("/playback", app.GetPlaybackSettings)
		r.Put("/playback", app.UpdatePlaybackSettings)
		r.With(app.RequireAdmin).Post("/scan/music", app.TriggerMusicScan)
		r.With(app.RequireAdmin).Post("/scan/movies", app.TriggerMovieScan)
	})
}

func (app *Application) registerMusicRoutes(r chi.Router) {
	r.Route("/music", func(r chi.Router) {
		r.Get("/stats", app.GetMusicStats)
		app.registerAlbumRoutes(r)
		app.registerMusicianRoutes(r)
		app.registerTrackRoutes(r)
		app.registerPlaylistRoutes(r)
		app.registerUserStatsRoutes(r)
	})
}

func (app *Application) registerAlbumRoutes(r chi.Router) {
	r.Route("/albums", func(r chi.Router) {
		r.Get("/", app.GetAlbumsAlphabetical)
		r.Get("/details/{id}", app.GetAlbumDetails)
		r.Get("/latest", app.GetLatestAlbums)
		r.With(app.RequireAdmin).Delete("/{id}", app.DeleteAlbum)
	})
}

func (app *Application) registerMusicianRoutes(r chi.Router) {
	r.Route("/musicians", func(r chi.Router) {
		r.Get("/", app.GetMusiciansAlphabetical)
		r.Get("/{id}", app.GetMusicianDetails)
	})
}

func (app *Application) registerTrackRoutes(r chi.Router) {
	r.Route("/tracks", func(r chi.Router) {
		r.Get("/", app.GetTracksAlphabetical)
		r.Get("/shuffle", app.GetShuffleTracks)
		r.Get("/details/{id}", app.GetTrackByID)
		r.Get("/{id}/stream", app.StreamTrack)
		r.Head("/{id}/stream", app.StreamTrack)
		r.Post("/{id}/like", app.ToggleLikeTrack)
		r.Get("/liked", app.GetLikedTracks)
		r.Get("/liked-ids", app.GetLikedTrackIDsForUser)
	})
}

func (app *Application) registerPlaylistRoutes(r chi.Router) {
	r.Route("/playlists", func(r chi.Router) {
		r.Get("/", app.GetPlaylists)
		r.Post("/", app.CreatePlaylist)
		r.Get("/{id}", app.GetPlaylist)
		r.Put("/{id}", app.UpdatePlaylist)
		r.Delete("/{id}", app.DeletePlaylist)
		r.Get("/{id}/tracks", app.GetPlaylistTracks)
		r.Post("/{id}/tracks", app.AddTracksToPlaylist)
		r.Delete("/{id}/tracks/{trackId}", app.RemoveTrackFromPlaylist)
		r.Put("/{id}/tracks/reorder", app.ReorderPlaylistTracks)
		r.Get("/{id}/collaborators", app.GetPlaylistCollaborators)
		r.Post("/{id}/collaborators", app.AddCollaborator)
		r.Delete("/{id}/collaborators/{userId}", app.RemoveCollaborator)
	})
}

func (app *Application) registerUserStatsRoutes(r chi.Router) {
	r.Route("/user-stats", func(r chi.Router) {
		r.Post("/play", app.RecordPlayEvent)
		r.Get("/overview", app.GetUserListeningStats)
		r.Get("/top-tracks", app.GetUserTopTracks)
		r.Get("/top-musicians", app.GetUserTopMusicians)
		r.Get("/top-genres", app.GetUserTopGenres)
		r.Get("/top-albums", app.GetUserTopAlbums)
		r.Get("/recently-played", app.GetUserRecentlyPlayed)
	})
}
