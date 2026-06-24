package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/DanielPopoola/span_ingestion/internal/store"
)

type IngestRequest struct {
	Spans []store.Span `json:"spans"`
}

type TraceListResponse struct {
	Total  int                  `json:"total"`
	Traces []store.TraceSummary `json:"traces"`
}

type Handler struct {
	store *store.Store
}

func NewHandler(store *store.Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) IngestSpans(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed request body", http.StatusBadRequest)
		return
	}
	if len(req.Spans) == 0 {
		http.Error(w, "spans empty, at least one span is required", http.StatusBadRequest)
		return
	}

	h.store.AddSpans(req.Spans)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) GetAllTraces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	parseInt64 := func(v string) (int64, error) {
		if v == "" {
			return 0, nil
		}
		return strconv.ParseInt(v, 10, 64)
	}

	after, err := parseInt64(q.Get("after"))
	if err != nil {
		http.Error(w, "invalid after", http.StatusBadRequest)
		return
	}

	before, err := parseInt64(q.Get("before"))
	if err != nil {
		http.Error(w, "invalid before", http.StatusBadRequest)
		return
	}

	limit := 20 // default
	if q.Get("limit") != "" {
		l, err := strconv.Atoi(q.Get("limit"))
		if err != nil || l < 1 || l > 1000 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = l
	}

	if after != 0 && before != 0 && after >= before {
		http.Error(w, "after must be < before", http.StatusBadRequest)
		return
	}

	results := h.store.GetAllTraces(after, before)

	total := len(results)
	if len(results) > limit {
		results = results[:limit]
	}

	resp := TraceListResponse{
		Total:  total,
		Traces: results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

}

func (h *Handler) GetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("id")

	if traceID == "" {
		http.Error(w, "missing trace id", http.StatusBadRequest)
		return
	}

	trace, err := h.store.GetTrace(traceID)
	if err != nil {
		http.Error(w, "trace not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trace)
}
