package store_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DanielPopoola/span_ingestion/internal/store"
)

func TestAddSpans_GroupsByTraceID(t *testing.T) {
	s := store.NewStore(30)

	spans := []store.Span{
		{SpanID: "s1", TraceID: "trace-abc", Service: "checkout", Operation: "buy", StartTime: time.Now().UnixNano(), EndTime: time.Now().UnixNano(), Status: "ok"},
		{SpanID: "s2", TraceID: "trace-abc", Service: "inventory", Operation: "check", StartTime: time.Now().UnixNano(), EndTime: time.Now().UnixNano(), Status: "ok"},
		{SpanID: "s3", TraceID: "trace-xyz", Service: "payment", Operation: "charge", StartTime: time.Now().UnixNano(), EndTime: time.Now().UnixNano(), Status: "ok"},
	}

	s.AddSpans(spans)

	trace, err := s.GetTrace("trace-abc")
	if err != nil {
		t.Fatalf("expected trace-abc to exist, got error: %v", err)
	}
	if len(trace.Spans) != 2 {
		t.Errorf("expected 2 spans, got %d", len(trace.Spans))
	}
}

func TestGetAllTraces_FiltersOutsideWindow(t *testing.T) {
	s := store.NewStore(30)

	now := time.Now().UnixNano()
	oldTime := time.Now().Add(-45 * time.Minute).UnixNano()

	spans := []store.Span{
		{SpanID: "s1", TraceID: "fresh", Service: "checkout", Operation: "buy", StartTime: now, EndTime: now, Status: "ok"},
		{SpanID: "s2", TraceID: "stale", Service: "checkout", Operation: "buy", StartTime: oldTime, EndTime: oldTime, Status: "ok"},
	}

	s.AddSpans(spans)

	traces := s.GetAllTraces(0, 0)
	if len(traces) != 1 {
		t.Errorf("expected 1 trace in window, got %d", len(traces))
	}
	if traces[0].TraceID != "fresh" {
		t.Errorf("expected fresh trace, got %s", traces[0].TraceID)
	}
}

func TestEvict_RemovesAgedOutSpans(t *testing.T) {
	s := store.NewStore(30)

	now := time.Now().UnixNano()
	oldTime := time.Now().Add(-45 * time.Minute).UnixNano()

	spans := []store.Span{
		{SpanID: "fresh", TraceID: "trace-1", Service: "checkout", Operation: "buy", StartTime: now, EndTime: now, Status: "ok"},
		{SpanID: "stale", TraceID: "trace-2", Service: "checkout", Operation: "buy", StartTime: oldTime, EndTime: oldTime, Status: "ok"},
	}

	s.AddSpans(spans)
	s.Evict()

	_, err := s.GetTrace("trace-2")
	if err == nil {
		t.Error("stale trace should have been evicted")
	}
}

func TestOutOfOrder_AssemblesCorrectly(t *testing.T) {
	s := store.NewStore(30)

	now := time.Now().UnixNano()
	rootID := "root-span"

	spans := []store.Span{
		{SpanID: "child-1", TraceID: "trace-1", ParentSpanID: &rootID, Service: "inventory", Operation: "check", StartTime: now, EndTime: now, Status: "ok"},
		{SpanID: "child-2", TraceID: "trace-1", ParentSpanID: &rootID, Service: "payment", Operation: "charge", StartTime: now, EndTime: now, Status: "ok"},
		{SpanID: rootID, TraceID: "trace-1", Service: "checkout", Operation: "buy", StartTime: now, EndTime: now, Status: "ok"},
	}

	s.AddSpans(spans)

	trace, err := s.GetTrace("trace-1")
	if err != nil {
		t.Fatalf("trace not found: %v", err)
	}
	if len(trace.Spans) != 3 {
		t.Errorf("expected 3 spans, got %d", len(trace.Spans))
	}

	var root *store.Span
	for i := range trace.Spans {
		if trace.Spans[i].ParentSpanID == nil {
			root = &trace.Spans[i]
			break
		}
	}
	if root == nil {
		t.Error("root span not found in assembled trace")
	}
	if root.SpanID != rootID {
		t.Errorf("wrong root span: got %s", root.SpanID)
	}
}

func TestOutOfOrder_AcrossBatches(t *testing.T) {
	s := store.NewStore(30)

	now := time.Now().UnixNano()
	rootID := "root-span"

	// Batch 1 — only children
	s.AddSpans([]store.Span{
		{SpanID: "child-1", TraceID: "trace-1", ParentSpanID: &rootID, Service: "inventory", Operation: "check", StartTime: now, EndTime: now, Status: "ok"},
	})

	// Root hasn't arrived yet — trace exists but no root
	trace, _ := s.GetTrace("trace-1")
	if len(trace.Spans) != 1 {
		t.Errorf("expected 1 span after first batch, got %d", len(trace.Spans))
	}

	// Batch 2 — root arrives
	s.AddSpans([]store.Span{
		{SpanID: rootID, TraceID: "trace-1", Service: "checkout", Operation: "buy", StartTime: now, EndTime: now, Status: "ok"},
	})

	// Now trace should be complete
	trace, _ = s.GetTrace("trace-1")
	if len(trace.Spans) != 2 {
		t.Errorf("expected 2 spans after second batch, got %d", len(trace.Spans))
	}
}

func TestEviction_LeavesWindowSpansIntact(t *testing.T) {
	s := store.NewStore(30)

	now := time.Now().UnixNano()
	oldTime := time.Now().Add(-45 * time.Minute).UnixNano()

	spans := []store.Span{
		{SpanID: "f1", TraceID: "fresh", Service: "checkout", Operation: "buy", StartTime: now, EndTime: now, Status: "ok"},
		{SpanID: "f2", TraceID: "fresh", Service: "inventory", Operation: "check", StartTime: now, EndTime: now, Status: "ok"},
		{SpanID: "s1", TraceID: "stale", Service: "payment", Operation: "charge", StartTime: oldTime, EndTime: oldTime, Status: "ok"},
	}

	s.AddSpans(spans)
	s.Evict()

	trace, err := s.GetTrace("fresh")
	if err != nil {
		t.Fatalf("fresh trace should survive eviction: %v", err)
	}
	if len(trace.Spans) != 2 {
		t.Errorf("expected 2 fresh spans, got %d", len(trace.Spans))
	}

	_, err = s.GetTrace("stale")
	if err == nil {
		t.Error("stale trace should have been evicted")
	}
}

func TestConcurrentWrites_NoCorruption(t *testing.T) {
	s := store.NewStore(30)

	workers := 20
	spansPerWorker := 10
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			spans := make([]store.Span, spansPerWorker)
			for j := 0; j < spansPerWorker; j++ {
				spans[j] = store.Span{
					SpanID:    fmt.Sprintf("w%d-s%d", workerID, j),
					TraceID:   fmt.Sprintf("trace-%d", workerID),
					Service:   "checkout",
					Operation: "buy",
					StartTime: time.Now().UnixNano(),
					EndTime:   time.Now().UnixNano(),
					Status:    "ok",
				}
			}
			s.AddSpans(spans)
		}(i)
	}

	wg.Wait()

	totalSpans := 0
	for i := 0; i < workers; i++ {
		trace, err := s.GetTrace(fmt.Sprintf("trace-%d", i))
		if err != nil {
			t.Errorf("trace-%d missing: %v", i, err)
			continue
		}
		totalSpans += len(trace.Spans)
	}

	expected := workers * spansPerWorker
	if totalSpans != expected {
		t.Errorf("expected %d total spans, got %d", expected, totalSpans)
	}
}
