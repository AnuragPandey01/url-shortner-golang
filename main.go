package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	// "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UrlModel struct {
	id         int
	url        string
	shortCode string
}

type ShortenResponse struct {
	ShortUrl string `json:"short_url"`
}

var conn *pgxpool.Pool

//POST /api/shorten?q="{url}"
func CreateShortCode(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	
	if conn == nil {
		http.Error(w, "db connection not available", http.StatusInternalServerError)
		return
	}

	url := r.URL.Query().Get("q")
	ctx := context.Background()

	fmt.Print(url)
	
	if !strings.HasPrefix(url, "https://") {
		http.Error(w, "not a valid url", http.StatusBadRequest)
		return
	}

	//get id
	var id int
	if err := conn.QueryRow(ctx, "SELECT nextval(pg_get_serial_sequence('url', 'id'));").Scan(&id); err != nil {
		http.Error(w, "error db connection", http.StatusInternalServerError)
		return
	}

	now := time.Now().Unix() + int64(id)
	var i big.Int
	i.SetInt64(now)
	shortCode := i.Text(62)
	

	insertQuery := `
		INSERT INTO url (url, short_code) VALUES ($1, $2);
	`
	
	if _, err := conn.Exec(context.Background(), insertQuery, url, shortCode); err != nil {
		http.Error(w, "error generating shortcode", http.StatusInternalServerError)
		return
	}

	payload := ShortenResponse{
		ShortUrl: shortCode,
	}

	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

//GET /api/r/{shortcode}
func ResolveShortCode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if conn == nil {
		http.Error(w, "db connection not available", http.StatusInternalServerError)
		return
	}

	shortCode := r.PathValue("shortCode")
	ctx := context.Background()

	if shortCode == "" {
		http.Error(w, "short code missing", http.StatusBadRequest)
		return
	}

	var longUrl string
	var query = `
		SELECT url
		FROM url
		WHERE short_code=$1
		LIMIT 1;
	`

	fmt.Print(shortCode+"\n")
	if err := conn.QueryRow(ctx, query, shortCode).Scan(&longUrl); err != nil {
		fmt.Print(err)
		http.Error(w, "error db connection", http.StatusInternalServerError)
	}
	
	http.Redirect(w, r, longUrl, http.StatusFound)
}

func registerHandlers(){
	http.HandleFunc("/api/shorten",CreateShortCode)
	http.HandleFunc("/r/{shortCode}",ResolveShortCode)
}

func main() {

	ctx := context.Background()

	var err error
	conn, err = pgxpool.New(ctx, "postgres://user:password@db:5432/url-db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

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
