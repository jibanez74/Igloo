package main

import (
	"database/sql"

	"igloo/cmd/internal/helpers"
)

// userSummary is the public face of a user carried by resources other users
// can see (watch rooms, playlists): no email, admin flag or PIN. It is the
// UserSummary schema in docs/openapi.json.
type userSummary struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
}

func newUserSummary(id int64, name string, avatar sql.NullString) userSummary {
	return userSummary{ID: id, Name: name, Avatar: helpers.StringPtrFromNull(avatar)}
}
