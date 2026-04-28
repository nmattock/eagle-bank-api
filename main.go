package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	accountspostgres "eagle-bank-api/internal/accounts/postgres"
	"eagle-bank-api/internal/auth"
	"eagle-bank-api/internal/httpapi"
	userspostgres "eagle-bank-api/internal/users/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultDatabaseURL = "postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable"

func main() {
	dsn := getenv("DATABASE_URL", defaultDatabaseURL)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	userRepo := userspostgres.NewRepository(db)
	accountRepo := accountspostgres.NewRepository(db)
	tokenService := auth.NewTokenService(getenv("JWT_SECRET", "dev-secret-change-me"), time.Hour)

	mux := http.NewServeMux()
	mux.Handle("/v1/accounts", httpapi.NewAccountHandler(accountRepo, tokenService))
	mux.Handle("/v1/accounts/", httpapi.NewAccountHandler(accountRepo, tokenService))
	mux.Handle("/v1/users", httpapi.NewUserHandler(userRepo, tokenService))
	mux.Handle("/v1/users/", httpapi.NewUserHandler(userRepo, tokenService))
	mux.Handle("/v1/auth/token", httpapi.NewAuthHandler(userRepo, tokenService))

	addr := getenv("HTTP_ADDR", ":8080")
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
