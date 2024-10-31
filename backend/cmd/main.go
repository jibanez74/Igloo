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
	"time"

	"os"

	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
)

type config struct {
	debug    bool
	repo     repository.Repo
	tmdb     tmdb.Tmdb
	wait     *sync.WaitGroup
	session  *scs.SessionManager
	infoLog  *log.Logger
	errorLog *log.Logger
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

	app.initSession()
	app.wait = &sync.WaitGroup{}
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

func (app *config) initSession() {
	session := scs.New()

	session.Store = redisstore.New(initRedis())
	session.Lifetime = 1 * time.Hour
	session.Cookie.Persist = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = true
	session.Cookie.Name = os.Getenv("COOKIE_NAME")
	session.Cookie.Domain = os.Getenv("COOKIE_DOMAIN")
	session.Cookie.HttpOnly = true

	app.session = session
}

func initRedis() *redis.Pool {
	redisPool := &redis.Pool{
		MaxIdle: 10,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", os.Getenv("REDIS_ADDR"))
		},
	}

	return redisPool
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
	app.wait.Wait()
	app.infoLog.Println("closing channels and shutting down application...")
}
