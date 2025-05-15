package main
import (
	"log"
	"net/http"
	"os"  
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow all for demo
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		
		version := os.Getenv("VERSION")
		if version == "" {
			version = "v1" // Default to v1 if not specified
		}
		w.Header().Set("X-API-Version", version)
		
		// For OPTIONS requests, return 200 OK with appropriate headers
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
