package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DBPool *pgxpool.Pool
	Router *chi.Mux
}
