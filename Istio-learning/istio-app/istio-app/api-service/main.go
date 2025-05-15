package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize DB
	InitDB()

	// Create router
	r := mux.NewRouter()

	// API routes
	r.HandleFunc("/api/data", GetDataHandler).Methods("GET")
	r.HandleFunc("/api/data", CreateDataHandler).Methods("POST")
	r.HandleFunc("/api/metrics", MetricsHandler).Methods("GET")

	// Middleware
	r.Use(LoggingMiddleware)
	r.Use(CorsMiddleware)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("API service running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
