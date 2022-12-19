package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog/log"

	hstats "github.com/michele/hourly-stats"
	"github.com/michele/hourly-stats/server"
)

func main() {
	path := os.Getenv("HSTATS_DB_PATH")
	if len(path) == 0 {
		log.Panic().Msg("Path to DB not set")
	}
	tkn := os.Getenv("HSTATS_AUTH_TOKEN")
	if len(tkn) == 0 {
		log.Panic().Msg("Auth token not set")
	}
	host := os.Getenv("HOST")
	if len(host) == 0 {
		host = "127.0.0.1"
	}
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	db, err := hstats.NewDB(path)
	if err != nil {
		log.Panic().Err(err).Msgf("Couldn't load DB: %+v", err)
	}

	srv, err := server.New(host, port, tkn, db)
	if err != nil {
		log.Panic().Err(err).Msgf("Couldn't create server: %+v", err)
	}
	var wait sync.WaitGroup
	wait.Add(2)
	srv.Start(&wait)
	db.Start()
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGINT, syscall.SIGTERM)
	<-gracefulStop
	log.Print("Closing...")
	go func() {
		err := db.Close()
		if err != nil {
			log.Panic().Err(err).Msgf("Couldn't close DB: %+v", err)
		}
		wait.Done()
	}()
	go func() {
		srv.Stop(context.Background())
	}()
	wait.Wait()
}
