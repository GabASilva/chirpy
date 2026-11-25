package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/GabASilva/chirpy/internal/database"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	dbQueries := database.New(db)
	apiCfg := &apiConfig{dbQueries: dbQueries}
	serverMux := http.NewServeMux()
	serverMux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	serverMux.HandleFunc("GET /api/healthz", handleRequest)
	serverMux.HandleFunc("GET /admin/metrics", apiCfg.handleMetrics)
	serverMux.HandleFunc("POST /admin/reset", apiCfg.handleResetMetrics)
	serverMux.HandleFunc("POST /api/validate_chirp", handleChirp)

	server := &http.Server{
		Addr:    ":8080",
		Handler: serverMux,
	}
	http.ListenAndServe(server.Addr, server.Handler)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handleMetrics(w http.ResponseWriter, r *http.Request) {
	hits := cfg.fileserverHits.Load()
	html := fmt.Sprintf(`<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
		</body>
	</html>`, hits)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

func (cfg *apiConfig) handleResetMetrics(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
}

func handleChirp(w http.ResponseWriter, r *http.Request) {
	type chirp struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := chirp{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if len(params.Body) > 140 {
		respondWithJSON(w, 400, map[string]string{"error": "Chirp body exceeds 140 characters"})
		return
	}
	words := strings.Split(params.Body, " ")
	for i, word := range words {
		if strings.ToLower(word) == "kerfuffle" || strings.ToLower(word) == "sharbert" || strings.ToLower(word) == "fornax" {
			word = "****"
		}
		words[i] = word
	}
	params.Body = strings.Join(words, " ")
	respondWithJSON(w, 200, map[string]string{"cleaned_body": params.Body})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}
