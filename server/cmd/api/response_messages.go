package main

// Messages said by more than one handler file. A handler that needs to say
// something different keeps its own local constant, and a message owned by a
// single feature lives in that feature's file instead.
const (
	internalServerErrorMessage = "The server encountered an unexpected error"
	notAuthorizedMessage       = "not authorized"
	invalidRequestBodyMessage  = "invalid request body"
	accessDeniedMessage        = "access denied"
	tooManyAttemptsMessage     = "too many attempts, please try again later"

	// Said by the movie and the episode technical-details endpoints.
	fetchTechnicalDetailsMessage = "failed to fetch technical details"
)

// Log lines repeated across handlers that share no feature file.
const beginTransactionLogMessage = "failed to begin transaction"
