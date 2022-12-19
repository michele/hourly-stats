package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	hstats "github.com/michele/hourly-stats"
)

type Server struct {
	r    *chi.Mux
	srv  *http.Server
	db   *hstats.DB
	host string
	port string
}

func New(host, port string, tkn string, d *hstats.DB) (*Server, error) {
	s := &Server{
		host: host,
		port: port,
		db:   d,
	}
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(auth(tkn))

	r.Post("/stats/{bucket:[a-z0-9-_]+}/{key:[a-z0-9-_]+}", s.postStat)
	r.Get("/stats/{bucket:[a-z0-9-_]+}", s.getReport)
	s.r = r
	s.srv = &http.Server{
		Addr:    fmt.Sprintf("%s:%s", s.host, s.port),
		Handler: r,
	}
	return s, nil
}

func (s *Server) Start(wait *sync.WaitGroup) {
	go func() {
		log.Printf("Listening on http://%s:%s", s.host, s.port)

		if err := s.srv.ListenAndServe(); err != nil {
			log.Printf("HTTP server stopped: %+v", err)
		}
		wait.Done()
	}()
}

func (s *Server) Stop(ctx context.Context) {
	go func() {
		s.srv.Shutdown(ctx)
	}()
}

func (s *Server) postStat(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")
	s.db.Incr(fmt.Sprintf("%s.%s", bucket, key))
	w.WriteHeader(200)
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	rep := s.db.Report(bucket)
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(rep)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(200)
}

func auth(tkn string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != tkn {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
