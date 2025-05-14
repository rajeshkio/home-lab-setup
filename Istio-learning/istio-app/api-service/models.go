package main

import "time"

type APIData struct {
	Message   string    `json:"message"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

type Record struct {
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
}
