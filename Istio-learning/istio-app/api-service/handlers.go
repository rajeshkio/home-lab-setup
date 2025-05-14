package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	requestCount int
	errorCount   int
	metricsLock  sync.Mutex
)

func GetDataHandler(w http.ResponseWriter, r *http.Request) {
	trackRequest()

	// Get version from environment, default to v1 if not set
	version := os.Getenv("VERSION")
	if version == "" {
		version = "v1"
	}

	data := APIData{
		Message:   "Hello from API Service",
		Version:   version,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(data)
}

func CreateDataHandler(w http.ResponseWriter, r *http.Request) {
	trackRequest()

	// Get version from environment, default to v1 if not set
	version := os.Getenv("VERSION")
	if version == "" {
		version = "v1"
	}

	var data Record
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		trackError()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	data.Timestamp = time.Now()
	if err := InsertRecord(data); err != nil {
		trackError()
		http.Error(w, "Failed to insert record", http.StatusInternalServerError)
		return
	}

	response := APIData{
		Message:   "Record created successfully",
		Version:   version,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	// Get version from environment, default to v1 if not set
	version := os.Getenv("VERSION")
	if version == "" {
		version = "v1"
	}

	metrics := map[string]interface{}{
		"requests":  requestCount,
		"errors":    errorCount,
		"version":   version,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(metrics)
}

func trackRequest() {
	metricsLock.Lock()
	defer metricsLock.Unlock()
	requestCount++
}

func trackError() {
	metricsLock.Lock()
	defer metricsLock.Unlock()
	errorCount++
}
