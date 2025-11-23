package main

const (
	DEFAULT_PASSWORD = "AdminPassword"

	// constants for ffprobe
	CODEC_TYPE_AUDIO = "audio"
	CODEC_TYPE_VIDEO = "video"

	// constants for genre types
	MUSIC_GENRE_TYPE = "music"
	MOVIE_GENRE_TYPE = "movie"
	MUSIC_TYPE       = "music"

	// constants for http status messages
	INVALID_REQUEST_BODY  = "error while parsing the request body"
	INVALID_CREDENTIALS   = "invalid email or password"
	NOT_AUTHORIZED        = "not authorized"
	FORBIDDeN             = "forbidden action"
	INTERNAL_SERVER_ERROR = "the server encountered an error. please try again later. if the problem persists, contact the server's administrator and report the issue"

	// scanner constants
	BATCH_SIZE = 100

	// auth constants
	COOKIE_USER_ID  = "user_id"
	COOKIE_IS_ADMIN = "is_admin"
	COOKIE_EMAIL    = "email"
	SUCCESS_LOGIN   = "User %s with id of %d signed successfully."
	SUCCESS_LOGOUT  = "Successfully signed out of your account!"
)
