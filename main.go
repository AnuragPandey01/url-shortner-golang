package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type UrlModel struct {
	id        int
	url       string
	shortCode string
}

type ShortenResponse struct {
	ShortUrl string `json:"short_url"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string, err error) {
	payload := ErrorResponse{Code: code, Error: message}
	if err != nil {
		payload.Details = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

var conn *pgx.Conn

// POST /api/shorten?q="{url}"
func CreateShortCode(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	if conn == nil {
		writeError(w, http.StatusServiceUnavailable, "db_unavailable", "db connection not available", nil)
		return
	}

	url := r.URL.Query().Get("q")
	ctx := r.Context()

	if !strings.HasPrefix(url, "https://") {
		writeError(w, http.StatusBadRequest, "invalid_url", "not a valid url", nil)
		return
	}

	//get id
	var id int
	if err := conn.QueryRow(ctx, "SELECT nextval(pg_get_serial_sequence('url', 'id'));").Scan(&id); err != nil {
		writeError(w, http.StatusInternalServerError, "db_sequence_failed", "failed to allocate URL ID", err)
		return
	}

	now := time.Now().Unix() + int64(id)
	var i big.Int
	i.SetInt64(now)
	shortCode := i.Text(62)

	insertQuery := `
		INSERT INTO url (url, short_code) VALUES ($1, $2);
	`

	if _, err := conn.Exec(ctx, insertQuery, url, shortCode); err != nil {
		writeError(w, http.StatusInternalServerError, "db_insert_failed", "error generating shortcode", err)
		return
	}

	payload := ShortenResponse{
		ShortUrl: shortCode,
	}

	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(payload)
}

// GET /api/r/{shortcode}
func ResolveShortCode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if conn == nil {
		writeError(w, http.StatusServiceUnavailable, "db_unavailable", "db connection not available", nil)
		return
	}

	shortCode := r.PathValue("shortCode")
	ctx := r.Context()

	if shortCode == "" {
		writeError(w, http.StatusBadRequest, "missing_short_code", "short code missing", nil)
		return
	}

	var longUrl string
	var query = `
		SELECT url
		FROM url
		WHERE short_code=$1
		LIMIT 1;
	`

	if err := conn.QueryRow(ctx, query, shortCode).Scan(&longUrl); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "short_code_not_found", "short code not found", nil)
		} else {
			writeError(w, http.StatusInternalServerError, "db_lookup_failed", "failed to resolve short code", err)
		}
		return
	}

	http.Redirect(w, r, longUrl, http.StatusFound)
}

func registerHandlers() {
	http.HandleFunc("/api/shorten", CreateShortCode)
	http.HandleFunc("/r/{shortCode}", ResolveShortCode)
}

func main() {

	ctx := context.Background()

	var err error
	conn, err = pgx.Connect(ctx, "postgres://user:password@db:5432/url-db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	//create table
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS url (
			id BIGSERIAL PRIMARY KEY,
		 	url TEXT NOT NULL,
			short_code VARCHAR(10) UNIQUE NOT NULL
		);
	`
	conn.Exec(ctx, createTableQuery)

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "healthy!")
	})
	registerHandlers()

	err = http.ListenAndServe(":8080", nil)

	log.Fatal(err)

}
