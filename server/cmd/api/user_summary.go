package main

// userSummary is the public face of a user carried by resources other users
// can see (watch rooms, playlists): no email, admin flag or PIN. It is the
// UserSummary schema in docs/openapi.json.
type userSummary struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
}
