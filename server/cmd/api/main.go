package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Failed to load .env file: ", err)
	}

	app, err := InitApp()
	if err != nil {
		log.Fatal("unable to create app", err)
	}

	err = http.ListenAndServe(fmt.Sprintf(":%d", app.Settings.Port), app.InitRouter())
	if err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}

