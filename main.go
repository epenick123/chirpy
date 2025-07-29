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

	"github.com/epenick123/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

type User struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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
		Body   string `json:"body"`
		UserID string `json:"user_id"`  // Make sure this json tag is correct
	}
	
	req := r.Method
	if req == http.MethodPost {
		// Decode JSON from the request body
		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		err := decoder.Decode(&params)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		// Convert the UserID to a UUID
		userID, err := uuid.Parse(params.UserID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		user, err := cfg.db.GetUser(r.Context(), userID)
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
		
		newChirpID := uuid.New() // Generate the ID here
		createParams := database.CreateChirpParams{ // Use the generated struct
    		ID:     newChirpID,
    		Body:   cleanedText,
    		UserID: userID,
		}
		
		new_chirp, err := cfg.db.CreateChirp(r.Context(), createParams)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError , "Error creating chirp")
			return
		}

		respondWithJSON(w, http.StatusCreated, response{
			ID: new_chirp.ID,
			CreatedAt: new_chirp.CreatedAt,
			UpdatedAt: new_chirp.UpdatedAt,
			Body: new_chirp.Body,
			UserID: new_chirp.UserID.String(),
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
				ID: chirp.ID,
				CreatedAt: chirp.CreatedAt,
				UpdatedAt: chirp.UpdatedAt,
				Body: chirp.Body,
				UserID: chirp.UserID.String(),
			}
			formatted_slice = append(formatted_slice, formatted_chirp)
		}
		respondWithJSON(w,200,formatted_slice)
	} else {
		respondWithError(w, http.StatusBadRequest, "HTTP request error")
		return
	}
}

func (cfg *apiConfig) singleChirpHandler(w http.ResponseWriter, r *http.Request) {
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
	formatted_chirp := chirpResponse {
		ID: found_chirp.ID,
		CreatedAt: found_chirp.CreatedAt,
		UpdatedAt: found_chirp.UpdatedAt,
		Body: found_chirp.Body,
		UserID: found_chirp.UserID.String(),
	}
	respondWithJSON(w, 200, formatted_chirp)
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	// Define the expected request structure
	type parameters struct {
		Email string `json:"email"`
	}

	// Parse JSON from the request body
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), params.Email)
if err != nil {
    respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create user: %v", err))
    return
}

	responseUser := User{
		ID:        user.ID.String(),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	respondWithJSON(w, http.StatusCreated, responseUser)

}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	// Check if platform is "dev"
	// If not, return 403
	// Delete users and reset counter
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "This endpoint is only available in development mode")
		return
	}

	// Delete all users from the database
	err := cfg.db.DeleteAllUsers(r.Context())
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
	mux.HandleFunc("/api/users", apiCfg.createUserHandler)
	mux.HandleFunc("/api/chirps", apiCfg.chirpsHandler)
	mux.HandleFunc("/api/chirps/{chirpID}", apiCfg.singleChirpHandler)

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
