package main

import (
	"igloo/database"
)

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		panic(err)
	}
}
