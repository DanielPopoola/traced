package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/DanielPopoola/span_ingestion/internal/handler"
	"github.com/DanielPopoola/span_ingestion/internal/store"
)

func main() {
	window := getEnvAsInt("WINDOW_MINUTES", 30)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	s := store.NewStore(window)

	h := handler.NewHandler(s)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /spans", h.IngestSpans)
	mux.HandleFunc("GET /traces", h.GetAllTraces)
	mux.HandleFunc("GET /traces/{id}", h.GetTrace)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	if err := http.ListenAndServe(":"+port, corsHandler(mux)); err != nil {
		log.Fatal(err)
	}
}

func getEnvAsInt(key string, defaultVal int) int {
	val, err := strconv.Atoi(os.Getenv(key))
	if err != nil || val == 0 {
		return defaultVal
	}
	return val
}
