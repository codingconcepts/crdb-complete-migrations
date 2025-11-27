package server

import (
	"complete_migration/api/pkg/repo"
	"complete_migration/api/pkg/server/handlers"
	"complete_migration/api/pkg/server/middleware"
	"log"
	"net/http"

	"github.com/codingconcepts/errhandler"
)

type Server struct {
	mux  *http.ServeMux
	addr string

	repo repo.Repo
}

func New(addr string, repo repo.Repo) *Server {
	s := Server{
		mux:  http.NewServeMux(),
		addr: addr,
		repo: repo,
	}

	s.mux.Handle("/api/", s.v1Mux(repo))

	return &s
}

func (s *Server) v1Mux(repo repo.Repo) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("POST /customers", errhandler.Wrap(middleware.Log(handlers.OpenAccount(repo))))
	mux.Handle("GET /customers", errhandler.Wrap(middleware.Log(handlers.GetCustomers(repo))))
	mux.Handle("GET /customers/{id}", errhandler.Wrap(middleware.Log(handlers.GetCustomer(repo))))

	mux.Handle("GET /accounts/{id}", errhandler.Wrap(middleware.Log(handlers.GetBalance(repo))))
	mux.Handle("POST /accounts", errhandler.Wrap(middleware.Log(handlers.MakeTransfer(repo))))

	return http.StripPrefix("/api", mux)
}

func (s *Server) Start() error {
	server := &http.Server{Addr: s.addr, Handler: s.mux}

	log.Printf("listening on %q", s.addr)
	return server.ListenAndServe()
}
