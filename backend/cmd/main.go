package main

import (
	"errors"
	"igloo/cmd/database"
	"igloo/cmd/repository"
	"igloo/cmd/tmdb"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"os"
)

type config struct {
	debug        bool
	repo         repository.Repo
	tmdb         tmdb.Tmdb
	wg           *sync.WaitGroup
	infoLog      *log.Logger
	errorLog     *log.Logger
	cookieSecret string
	cookieDomain string
}

func main() {
	var app config

	debug, err := strconv.ParseBool(os.Getenv("DEBUG"))
	if err != nil {
		panic(err)
	}
	app.debug = debug

	db, err := database.New()
	if err != nil {
		panic(err)
	}
	app.repo = repository.New(db)

	tmdbKey := os.Getenv("TMDB_API_KEY")
	if tmdbKey == "" {
		panic(errors.New("TMDB_API_KEY is not set"))
	}
	app.tmdb = tmdb.New(tmdbKey)

	secret := os.Getenv("COOKIE_SECRET")
	if secret == "" {
		panic(errors.New("COOKIE_SECRET is not set"))
	}
	app.cookieSecret = secret

	cookieDomain := os.Getenv("COOKIE_DOMAIN")
	if cookieDomain == "" {
		cookieDomain = "localhost"
	}
	app.cookieDomain = cookieDomain

	app.wg = &sync.WaitGroup{}
	app.infoLog = log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	app.errorLog = log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	go app.listenForShutdown()

	app.serve()
}

func (app *config) serve() {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}

	srv := &http.Server{
		Addr:    port,
		Handler: app.routes(),
	}

	app.infoLog.Println("Starting web server...")

	err := srv.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func (app *config) listenForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.shutdown()
	os.Exit(0)
}

func (app *config) shutdown() {
	app.infoLog.Println("would run cleanup tasks...")
	app.wg.Wait()
	app.infoLog.Println("closing channels and shutting down application...")
}
