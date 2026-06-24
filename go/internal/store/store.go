package store

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type Span struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID *string           `json:"parent_span_id,omitempty"`
	Service      string            `json:"service"`
	Operation    string            `json:"operation"`
	Status       string            `json:"status"`
	StartTime    int64             `json:"start_time"`
	EndTime      int64             `json:"end_time"`
	Tags         map[string]string `json:"tags,omitempty"`
}

type Trace struct {
	TraceID  string `json:"trace_id"`
	Spans    []Span `json:"spans"`
	HasError bool   `json:"-"`
}

type TraceSummary struct {
	TraceID       string `json:"trace_id"`
	RootService   string `json:"root_service"`
	RootOperation string `json:"root_operation"`
	SpanCount     int    `json:"span_count"`
	DurationMs    int    `json:"duration_ms"`
	StartTime     int64  `json:"start_time"`
	Status        string `json:"status"`
}

type Store struct {
	mu     sync.Mutex
	traces map[string]*Trace
	window int // WINDOW_MINUTES
}

func NewStore(windowMinutes int) *Store {
	s := &Store{
		traces: make(map[string]*Trace),
		window: windowMinutes,
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s.Evict()
		}
	}()

	return s
}

func (s *Store) AddSpans(spans []Span) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(s.window) * time.Minute).UnixNano()

	for i := range spans {
		span := spans[i]
		if span.StartTime < cutoff {
			continue
		}
		trace, exists := s.traces[span.TraceID]
		if !exists {
			trace = &Trace{
				TraceID: span.TraceID,
				Spans:   []Span{},
			}
			s.traces[span.TraceID] = trace
		}

		trace.Spans = append(trace.Spans, span)
		if span.Status == "error" {
			trace.HasError = true
		}
	}
}

func (s *Store) GetTrace(traceID string) (*Trace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	trace, exists := s.traces[traceID]
	if !exists {
		return nil, fmt.Errorf("trace not found")
	}

	traceCopy := *trace
	traceCopy.Spans = append([]Span(nil), trace.Spans...)
	return &traceCopy, nil
}

func (s *Store) GetAllTraces(after, before int64) []TraceSummary {
	s.mu.Lock()

	traces := make([]Trace, 0, len(s.traces))
	for _, t := range s.traces {
		copyTrace := *t
		copyTrace.Spans = append([]Span(nil), t.Spans...)
		traces = append(traces, copyTrace)
	}

	s.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(s.window) * time.Minute).UnixNano()
	var results []TraceSummary

	for _, trace := range traces {
		summary, ok := summarize(trace)
		if !ok {
			continue
		}

		if summary.StartTime < cutoff {
			continue
		}
		if after != 0 && summary.StartTime < after {
			continue
		}

		if before != 0 && summary.StartTime >= before {
			continue
		}

		results = append(results, summary)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].StartTime > results[j].StartTime
	})

	return results
}

func (s *Store) Evict() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(s.window) * time.Minute).UnixNano()

	for traceID, trace := range s.traces {

		// filter spans in-place
		var fresh []Span
		for _, span := range trace.Spans {
			if span.StartTime >= cutoff {
				fresh = append(fresh, span)
			}
		}
		trace.Spans = fresh

		// remove trace if empty
		if len(trace.Spans) == 0 {
			delete(s.traces, traceID)
		}
	}
}

func summarize(trace Trace) (TraceSummary, bool) {
	var root *Span

	for i := range trace.Spans {
		if trace.Spans[i].ParentSpanID == nil {
			root = &trace.Spans[i]
			break
		}
	}

	if root == nil {
		return TraceSummary{}, false
	}

	status := "ok"
	if trace.HasError {
		status = "error"
	}

	durationMs := int((root.EndTime - root.StartTime) / 1_000_000)

	return TraceSummary{
		TraceID:       trace.TraceID,
		RootService:   root.Service,
		RootOperation: root.Operation,
		SpanCount:     len(trace.Spans),
		DurationMs:    durationMs,
		StartTime:     root.StartTime,
		Status:        status,
	}, true
}
