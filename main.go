package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/healthz", healthzHandler)

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}

func healthzHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	w.WriteHeader(200)

	w.Write([]byte("OK"))

}
