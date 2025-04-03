package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	// Reset the counter back to 0
	cfg.fileserverHits.Store(0)

	// Set the header and status code
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Write a response if needed
	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) validationHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	type response struct {
		CleanedBody string `json:"cleaned_body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	// Clean the text and respond with the cleaned version
	cleanedText := cleanProfaneWords(params.Body)
	respondWithJSON(w, http.StatusOK, response{CleanedBody: cleanedText})
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
	apiCfg := apiConfig{}
	mux := http.NewServeMux()

	// Serve files from the root directory
	fileServer := http.FileServer(http.Dir("."))

	// Strip the /app prefix for file serving
	handler := http.StripPrefix("/app", fileServer)

	// Register the handler for /app - this will catch both /app and /app/
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))

	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	mux.HandleFunc("GET /api/healthz", healthzHandler)
	mux.HandleFunc("POST /api/validate_chirp", apiCfg.validationHandler)

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
