package handlers

import "igloo/cmd/repository"

type Handlers struct {
	repo *repository.Repo
}

func New(repo *repository.Repo) *Handlers {
	return &Handlers{
		repo: repo,
	}
}
