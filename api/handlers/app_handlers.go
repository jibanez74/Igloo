// description: defines a data struct for handlers
package handlers

type AppHandler struct {
	UserHandler       *userHandler
	MoodHandler       *moodHandler
	MusicGenreHandler *musicGenreHandler
	MusicianHandler   *musicianHandler
	AlbumHandler      *albumHandler
	TrackHandler      *trackHandler
}
