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
	"time"

	"github.com/GabASilva/chirpy/internal/auth"
	"github.com/GabASilva/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	dbQueries := database.New(db)
	apiCfg := &apiConfig{dbQueries: dbQueries, platform: platform}
	serverMux := http.NewServeMux()
	serverMux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	serverMux.HandleFunc("GET /api/healthz", handleRequest)
	serverMux.HandleFunc("GET /admin/metrics", apiCfg.handleMetrics)
	serverMux.HandleFunc("POST /admin/reset", apiCfg.handleResetMetrics)
	serverMux.HandleFunc("POST /api/users", apiCfg.handleUserCreation)
	serverMux.HandleFunc("POST /api/chirps", apiCfg.handleChirp)
	serverMux.HandleFunc("GET /api/chirps", apiCfg.handleRetrieveChirps)
	serverMux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handleRetrieveChirpByID)
	serverMux.HandleFunc("POST /api/login", apiCfg.handleLogin)

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
	platform       string
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
	if cfg.platform != "dev" {
		respondWithJSON(w, 403, map[string]string{"error": "Forbidden"})
		return
	}
	cfg.fileserverHits.Store(0)
	ctx := r.Context()
	sqlc := cfg.dbQueries
	err := sqlc.DeleteAllUsers(ctx)
	if err != nil {
		respondWithJSON(w, 500, map[string]string{"error": "Failed to delete users"})
		return
	}
	respondWithJSON(w, 200, map[string]string{"message": "Metrics and users reset successfully"})
}

type chirp struct {
	Id         uuid.UUID `json:"id"`
	Created_At time.Time `json:"created_at"`
	Updated_At time.Time `json:"updated_at"`
	Body       string    `json:"body"`
	User_id    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) handleChirp(w http.ResponseWriter, r *http.Request) {

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
	//respondWithJSON(w, 200, map[string]string{"cleaned_body": params.Body})

	sqlc := cfg.dbQueries
	createdChirp, err := sqlc.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   params.Body,
		UserID: params.User_id,
	})
	if err != nil {
		respondWithJSON(w, 500, map[string]string{"error": "Failed to create chirp"})
		return
	}

	newChirp := chirp{
		Id:         createdChirp.ID,
		Created_At: createdChirp.CreatedAt,
		Updated_At: createdChirp.UpdatedAt,
		Body:       createdChirp.Body,
		User_id:    createdChirp.UserID,
	}

	respondWithJSON(w, 201, newChirp)
}

func (cfg *apiConfig) handleRetrieveChirps(w http.ResponseWriter, r *http.Request) {
	sqlc := cfg.dbQueries
	chirps, err := sqlc.RetrieveAllChirps(r.Context())
	if err != nil {
		respondWithJSON(w, 500, map[string]string{"error": "Failed to retrieve chirps"})
		return
	}

	chirpList := []chirp{}
	for _, c := range chirps {
		chirpList = append(chirpList, chirp{
			Id:         c.ID,
			Created_At: c.CreatedAt,
			Updated_At: c.UpdatedAt,
			Body:       c.Body,
			User_id:    c.UserID,
		})
	}

	respondWithJSON(w, 200, chirpList)
}

func (cfg *apiConfig) handleRetrieveChirpByID(w http.ResponseWriter, r *http.Request) {
	chirpId := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpId)
	if err != nil {
		respondWithJSON(w, 400, map[string]string{"error": "Invalid chirp ID"})
		return
	}

	sqlc := cfg.dbQueries
	chirps, err := sqlc.RetrieveChirpsById(r.Context(), chirpID)
	if err != nil {
		respondWithJSON(w, 404, map[string]string{"error": "Failed to retrieve chirp"})
		return
	}

	newChirp := chirp{
		Id:         chirps.ID,
		Created_At: chirps.CreatedAt,
		Updated_At: chirps.UpdatedAt,
		Body:       chirps.Body,
		User_id:    chirps.UserID,
	}

	respondWithJSON(w, 200, newChirp)
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

type User struct {
	Id         uuid.UUID `json:"id"`
	Created_At time.Time `json:"created_at"`
	Updated_At time.Time `json:"updated_at"`
	Email      string    `json:"email"`
}

func (cfg *apiConfig) handleUserCreation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type userRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := userRequest{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	sqlc := cfg.dbQueries
	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithJSON(w, 500, map[string]string{"error": "Failed to hash password"})
		return
	}
	userInfo := database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	}
	createdUser, err := sqlc.CreateUser(ctx, userInfo)
	if err != nil {
		respondWithJSON(w, 500, map[string]string{"error": "Failed to create user"})
		return
	}
	newUser := User{
		Id:         createdUser.ID,
		Created_At: createdUser.CreatedAt,
		Updated_At: createdUser.UpdatedAt,
		Email:      createdUser.Email,
	}
	respondWithJSON(w, 201, newUser)
}

func (cfg *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type loginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := loginRequest{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	login, err := cfg.dbQueries.RetrieveUserByEmail(ctx, params.Email)
	if err != nil {
		respondWithJSON(w, 401, map[string]string{"error": "Incorrect email or password"})
		return
	}
	isMatch, err := auth.CheckPasswordHash(params.Password, login.HashedPassword)
	if err != nil || !isMatch {
		respondWithJSON(w, 401, map[string]string{"error": "Incorrect email or password"})
		return
	}

	newUser := User{
		Id:         login.ID,
		Created_At: login.CreatedAt,
		Updated_At: login.UpdatedAt,
		Email:      login.Email,
	}
	respondWithJSON(w, 200, newUser)
}
