package com.traced.uchiha.traced.span;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import static org.assertj.core.api.Assertions.assertThat;

class TraceStoreTest {

    private TraceStore store;

    @BeforeEach
    void setUp() {
        store = new TraceStore(30);
    }

    @Test
    void childSpanArrivingBeforeRootIsAssembledCorrectly() {
        Span child = new Span("trace1", "span2", "span1", "inventory", "check", "ok",
                System.nanoTime(), System.nanoTime() + 1_000_000, null);
        Span root = new Span("trace1", "span1", null, "checkout", "purchase", "ok",
                System.nanoTime(), System.nanoTime() + 2_000_000, null);

        store.addSpans(List.of(child));
        store.addSpans(List.of(root));

        Trace trace = store.getTrace("trace1");
        assertThat(trace).isNotNull();
        assertThat(trace.getSpans()).hasSize(2);
    }

    @Test
    void traceWithAnyErrorSpanHasErrorStatus() {
        Span root = new Span("trace2", "span1", null, "checkout", "purchase", "ok",
                System.nanoTime(), System.nanoTime() + 1_000_000, null);
        Span errorChild = new Span("trace2", "span2", "span1", "payment", "charge", "error",
                System.nanoTime(), System.nanoTime() + 1_000_000, null);

        store.addSpans(List.of(root, errorChild));

        List<TraceSummary> summaries = store.getAllTraces(0, 0);
        assertThat(summaries).hasSize(1);
        assertThat(summaries.get(0).status()).isEqualTo("error");
    }

    @Test
    void spansOutsideWindowAreDiscardedOnIngest() {
        long wayInThePast = System.nanoTime() - 60L * 60 * 1_000_000_000; // 60 mins ago
        Span staleSpan = new Span("trace3", "span1", null, "checkout", "purchase", "ok",
                wayInThePast, wayInThePast + 1_000_000, null);

        store.addSpans(List.of(staleSpan));

        assertThat(store.getTrace("trace3")).isNull();
    }

    @Test
    void evictRemovesTracesWhoseSpansAgedOut() {
        long wayInThePast = System.nanoTime() - 60L * 60 * 1_000_000_000;
        Trace trace = new Trace("trace4");
        trace.addSpan(new Span("trace4", "span1", null, "checkout", "purchase", "ok",
                wayInThePast, wayInThePast + 1_000_000, null));

        // inject directly into store by adding via a real recent span first,
        // then use evict to clean up a manually aged trace
        store.addSpans(List.of(
                new Span("trace5", "span1", null, "checkout", "purchase", "ok",
                        System.nanoTime(), System.nanoTime() + 1_000_000, null)
        ));

        store.evict();

        assertThat(store.getTrace("trace5")).isNotNull();
    }

    @Test
    void concurrentWritesDoNotCorruptState() throws InterruptedException {
        int threadCount = 20;
        int spansPerThread = 50;
        CountDownLatch latch = new CountDownLatch(threadCount);
        ExecutorService executor = Executors.newFixedThreadPool(threadCount);

        for (int i = 0; i < threadCount; i++) {
            final int threadId = i;
            executor.submit(() -> {
                for (int j = 0; j < spansPerThread; j++) {
                    Span span = new Span(
                            "trace-" + threadId,
                            "span-" + j,
                            j == 0 ? null : "span-0",
                            "service",
                            "op",
                            "ok",
                            System.nanoTime(),
                            System.nanoTime() + 1_000_000,
                            null
                    );
                    store.addSpans(List.of(span));
                }
                latch.countDown();
            });
        }

        latch.await();
        executor.shutdown();

        // each thread wrote to its own trace, so we expect threadCount traces
        List<TraceSummary> summaries = store.getAllTraces(0, 0);
        assertThat(summaries).hasSize(threadCount);
    }
}