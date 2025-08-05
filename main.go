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

	"github.com/epenick123/chirpy/internal/auth"
	"github.com/epenick123/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	jwtsecret      string
	polkaKey       string
}

type User struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

type response struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"` // Corrected type and added tag
	UpdatedAt time.Time `json:"updated_at"` // Corrected type and added tag
	Body      string    `json:"body"`       // Added tag
	UserID    string    `json:"user_id"`    // Added tag
}

type chirpResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    string    `json:"user_id"`
}

func (cfg *apiConfig) chirpsHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	req := r.Method
	if req == http.MethodPost {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			respondWithError(w, 401, "Unauthorized")
			return
		}
		user_id, err := auth.ValidateJWT(token, cfg.jwtsecret)
		if err != nil {
			respondWithError(w, 401, "Unauthorized")
			return
		}

		// Decode JSON from the request body
		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		err = decoder.Decode(&params)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		user, err := cfg.db.GetUserByID(r.Context(), user_id)
		if err != nil {
			if err == sql.ErrNoRows {
				respondWithError(w, http.StatusNotFound, "User not found")
			} else {
				respondWithError(w, http.StatusInternalServerError, "Error fetching user")
			}
			return
		}

		// Example use of `user`:
		fmt.Printf("User email: %s\n", user.Email)

		fmt.Printf("%+v\n", cfg.db)

		// Clean profanity and return response
		cleanedText := cleanProfaneWords(params.Body)
		// Ensure chirp body length does not exceed 140 characters
		if len(cleanedText) > 140 {
			respondWithError(w, http.StatusBadRequest, "Chirp is too long")
			return
		}

		newChirpID := uuid.New()                    // Generate the ID here
		createParams := database.CreateChirpParams{ // Use the generated struct
			ID:     newChirpID,
			Body:   cleanedText,
			UserID: user_id,
		}

		new_chirp, err := cfg.db.CreateChirp(r.Context(), createParams)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error creating chirp")
			return
		}

		respondWithJSON(w, http.StatusCreated, response{
			ID:        new_chirp.ID,
			CreatedAt: new_chirp.CreatedAt,
			UpdatedAt: new_chirp.UpdatedAt,
			Body:      new_chirp.Body,
			UserID:    new_chirp.UserID.String(),
		})

	} else if req == http.MethodGet {
		chirps_slice := []database.Chirp{}
		formatted_slice := []chirpResponse{}
		chirps_slice, err := cfg.db.GetChirps(r.Context())

		if err != nil {
			respondWithError(w, http.StatusBadRequest, "HTTP request error")
			return
		}
		for _, chirp := range chirps_slice {
			formatted_chirp := chirpResponse{
				ID:        chirp.ID,
				CreatedAt: chirp.CreatedAt,
				UpdatedAt: chirp.UpdatedAt,
				Body:      chirp.Body,
				UserID:    chirp.UserID.String(),
			}
			formatted_slice = append(formatted_slice, formatted_chirp)
		}
		respondWithJSON(w, 200, formatted_slice)
	} else {
		respondWithError(w, http.StatusBadRequest, "HTTP request error")
		return
	}
}

func (cfg *apiConfig) singleChirpHandler(w http.ResponseWriter, r *http.Request) {
	req := r.Method
	if req == http.MethodDelete {

		header := r.Header.Get("Authorization") // Extract the token from the Authorization header
		parts := strings.Split(header, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondWithError(w, 401, "Unauthorized")
			return
		}

		token := parts[1]
		user_id, err := auth.ValidateJWT(token, cfg.jwtsecret)
		if err != nil {
			respondWithError(w, 401, "Unauthorized")
			return
		}

		path_value := r.PathValue("chirpID")
		requested_id, err := uuid.Parse(path_value)
		if err != nil {
			respondWithError(w, 404, "Chirp UUID error")
			return
		}

		to_delete, err := cfg.db.GetChirp(r.Context(), requested_id)
		if err != nil {
			respondWithError(w, 404, "Chirp not found")
			return
		}

		// Check if the authenticated user owns this chirp
		if to_delete.UserID != user_id {
			respondWithError(w, 403, "Forbidden")
			return
		}
		params := database.DeleteChirpByIDParams{
			ID:     to_delete.ID,
			UserID: to_delete.UserID,
		}

		// User owns the chirp, so delete it
		err = cfg.db.DeleteChirpByID(r.Context(), params)
		if err != nil {
			respondWithError(w, 500, "Failed to delete chirp")
			return
		}

		// Success - return 204
		w.WriteHeader(204)
		return
	}

	if req == http.MethodGet {
		path_value := r.PathValue("chirpID")
		requested_id, err := uuid.Parse(path_value)
		if err != nil {
			respondWithError(w, 404, "Chirp UUID error")
			return
		}
		found_chirp, err := cfg.db.GetChirp(r.Context(), requested_id)
		if err == sql.ErrNoRows {
			respondWithError(w, 404, "Error retrieving Chirp")
			return
		} else if err != nil {
			respondWithError(w, 500, "Other Database Error")
			return
		}
		formatted_chirp := chirpResponse{
			ID:        found_chirp.ID,
			CreatedAt: found_chirp.CreatedAt,
			UpdatedAt: found_chirp.UpdatedAt,
			Body:      found_chirp.Body,
			UserID:    found_chirp.UserID.String(),
		}
		respondWithJSON(w, 200, formatted_chirp)
	}
}

func (cfg *apiConfig) usersHandler(w http.ResponseWriter, r *http.Request) {
	req := r.Method

	if req == http.MethodPost {
		// Define the expected request structure
		type parameters struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		}

		// Parse JSON from the request body
		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		err := decoder.Decode(&params)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		hashedPassword, err := auth.HashPassword(params.Password)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Could not hash password")
			return
		}

		user_params := database.CreateUserParams{
			Email:          params.Email,
			HashedPassword: hashedPassword,
		}
		user, err := cfg.db.CreateUser(r.Context(), user_params)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create user: %v", err))
			return
		}

		responseUser := User{
			ID:          user.ID.String(),
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
			Email:       user.Email,
			IsChirpyRed: user.IsChirpyRed,
		}

		respondWithJSON(w, http.StatusCreated, responseUser)
	}

	if req == http.MethodPut {

		type parameters struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		header := r.Header.Get("Authorization") // Extract the token from the Authorization header
		parts := strings.Split(header, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondWithError(w, 401, "Unauthorized")
			return
		}

		token := parts[1]
		user_id, err := auth.ValidateJWT(token, cfg.jwtsecret)
		if err != nil {
			respondWithError(w, 401, "Unauthorized")
			return
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		err = decoder.Decode(&params)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}
		// Now params.Email and params.Password are filled from the JSON body.

		hashedPassword, err := auth.HashPassword(params.Password)
		if err != nil {
			respondWithError(w, 500, "Internal Server Error")
			return
		}

		err = cfg.db.UpdateEmailAndPassword(r.Context(), database.UpdateEmailAndPasswordParams{
			Email:          params.Email,
			HashedPassword: hashedPassword,
			ID:             user_id,
		})
		if err != nil {
			respondWithError(w, 500, "Internal Server Error")
			return
		}

		updatedUser, err := cfg.db.GetUserByID(r.Context(), user_id)
		if err != nil {
			respondWithError(w, 500, "Internal Server Error")
			return
		}

		// Prepare response (hide password)
		responseUser := User{
			ID:          updatedUser.ID.String(),
			CreatedAt:   updatedUser.CreatedAt,
			UpdatedAt:   updatedUser.UpdatedAt,
			Email:       updatedUser.Email,
			IsChirpyRed: updatedUser.IsChirpyRed,
		}

		respondWithJSON(w, http.StatusOK, responseUser)
	}

}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	// Now params.Email and params.Password are filled from the JSON body.

	user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	err = auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	token, err := auth.MakeJWT(user.ID, cfg.jwtsecret, time.Hour) // always one hour
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create JWT")
		return
	}

	refresh_token, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create refresh token")
		return
	}

	responseUser := User{
		ID:           user.ID.String(),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refresh_token,
		IsChirpyRed:  user.IsChirpyRed,
	}

	expiresAt := time.Now().Add(60 * 24 * time.Hour) // 60 days from now
	_, err = cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refresh_token,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		// Handle error (don't return the token if DB insert fails!)
		respondWithError(w, http.StatusInternalServerError, "Failed to create refresh token")
		return
	}

	respondWithJSON(w, 200, responseUser)
}

func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization") // Extract the token from the Authorization header
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	refresh_token := parts[1]

	refreshTokenRow, err := cfg.db.GetRefreshToken(r.Context(), refresh_token)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	token, err := auth.MakeJWT(refreshTokenRow.UserID, cfg.jwtsecret, time.Hour)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	respondWithJSON(w, 200, map[string]string{
		"token": token,
	})

}

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	refresh_token := parts[1]

	err := cfg.db.RevokeRefreshToken(r.Context(), refresh_token)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	w.WriteHeader(204)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	// Check if platform is "dev"
	// If not, return 403
	// Delete users and reset counter
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "This endpoint is only available in development mode")
		return
	}

	err := cfg.db.DeleteAllRefreshTokens(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete refresh tokens: %v", err))
		return
	}

	// Then delete all users
	err = cfg.db.DeleteAllUsers(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to reset users: %v", err))
		return
	}

	// Reset the counter back to 0
	cfg.fileserverHits.Store(0)

	// Set the header and status code
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Get the current hit count
	count := cfg.fileserverHits.Load()

	// Set the Content-Type header
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Write the response
	w.WriteHeader(http.StatusOK)

	// Format the response according to the instructions: "Hits: x"
	// Use fmt.Sprintf to format the string
	w.Write([]byte(fmt.Sprintf(`<html>
  		<body>
    	<h1>Welcome, Chirpy Admin</h1>
    	<p>Chirpy has been visited %d times!</p>
  		</body>
		</html>`, count)))
}

func (cfg *apiConfig) webhooksHandler(w http.ResponseWriter, r *http.Request) {
	type event struct {
		Event string `json:"event"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}

	polkaKey, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	if polkaKey != cfg.polkaKey {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	events := event{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&events)
	if err != nil {
		respondWithError(w, 404, "Something went wrong")
		return
	}

	if events.Event == "user.upgraded" {
		user_id_string := events.Data.UserID
		user_id, err := uuid.Parse(user_id_string)
		if err != nil {
			respondWithError(w, 404, "Something went wrong")
			return
		}
		err = cfg.db.UpgradeToChirpyRed(r.Context(), user_id)
		if err != nil {
			respondWithError(w, 404, "User not found")
			return
		}
		w.WriteHeader(204)
		return
	} else {
		w.WriteHeader(204)
		return
	}
}

func cleanProfaneWords(text string) string {
	words := strings.Split(text, " ")
	for i, word := range words {
		if strings.ToLower(word) == "kerfuffle" || strings.ToLower(word) == "sharbert" || strings.ToLower(word) == "fornax" {
			words[i] = "****"
		}
	}

	cleanedText := strings.Join(words, " ")
	fmt.Println(cleanedText)
	return cleanedText
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJSON(w, code, map[string]string{"error": msg})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Connect to the database
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	// Create a new API config
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             database.New(db),
		platform:       os.Getenv("PLATFORM"),
		jwtsecret:      os.Getenv("JWT_SECRET"),
		polkaKey:       os.Getenv("POLKA_KEY"),
	}

	mux := http.NewServeMux()

	// Serve files from the root directory
	fileServer := http.FileServer(http.Dir("."))

	// Strip the /app prefix for file serving
	handler := http.StripPrefix("/app", fileServer)

	// Register the handler for /app - this will catch both /app and /app/
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))

	mux.HandleFunc("/admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("/admin/reset", apiCfg.resetHandler)
	mux.HandleFunc("/api/healthz", healthzHandler)
	mux.HandleFunc("/api/users", apiCfg.usersHandler)
	mux.HandleFunc("/api/chirps", apiCfg.chirpsHandler)
	mux.HandleFunc("/api/chirps/{chirpID}", apiCfg.singleChirpHandler)
	mux.HandleFunc("/api/login", apiCfg.loginHandler)
	mux.HandleFunc("/api/refresh", apiCfg.refreshHandler)
	mux.HandleFunc("/api/revoke", apiCfg.revokeHandler)
	mux.HandleFunc("/api/polka/webhooks", apiCfg.webhooksHandler)

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
