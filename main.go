package main

import (
	"fmt"
	"log"
	"net/http"
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

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
