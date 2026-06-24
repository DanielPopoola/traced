package com.traced.uchiha.traced.span;

import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;
import java.util.stream.Collectors;

@Component
public class TraceStore {
    
    private final Map<String, Trace> traces = new HashMap<>();
    private final ReadWriteLock lock = new ReentrantReadWriteLock();
    private final int windowMinutes;

    public TraceStore(@org.springframework.beans.factory.annotation.Value("${traced.window-minutes:30}") int windowMinutes) {
        this.windowMinutes = windowMinutes;
    }

    public void addSpans(List<Span> spans) {
        long cutoff = System.nanoTime() - TimeUnit.MINUTES.toNanos(windowMinutes);

        lock.writeLock().lock();
        try {
            for (Span span: spans) {
                if (span.startTime() < cutoff) {
                    continue;
                }
                traces.computeIfAbsent(span.traceId(), Trace::new).addSpan(span);
            }
        } finally  {
            lock.writeLock().unlock();
        }
    }

    public Trace getTrace(String traceId) {
        lock.readLock().lock();
        try {
            return traces.get(traceId);
        } finally {
            lock.readLock().unlock();
        }
    }

    public List<TraceSummary> getAllTraces(long after, long before) {
        long cutoff = System.nanoTime() - TimeUnit.MINUTES.toNanos(windowMinutes);

        List<Trace> snapshot;
        lock.readLock().lock();
        try {
            snapshot = new ArrayList<>(traces.values());
        } finally {
            lock.readLock().unlock();
        }

        return snapshot.stream()
            .map(this::summarize)
            .filter(Optional::isPresent)
            .map(Optional::get)
            .filter(s -> s.startTime() >= cutoff)
            .filter(s -> after == 0 || s.startTime() >= after)
            .filter(s -> before == 0 || s.startTime() < before)
            .sorted(Comparator.comparingLong(TraceSummary::startTime).reversed())
            .collect(Collectors.toList());


    }

    @Scheduled(fixedDelay = 30000)
    public void evict() {
        long cutoff = System.nanoTime() - TimeUnit.MINUTES.toNanos(windowMinutes);

        lock.writeLock().lock();
        try {
            for (Iterator<Trace> it = traces.values().iterator(); it.hasNext(); ) {
                Trace trace = it.next();
                trace.getSpans().removeIf(s -> s.startTime() < cutoff);
                if (trace.getSpans().isEmpty()) {
                    it.remove();
                }
            } 
        } finally {
            lock.writeLock().unlock();
        }

    }
    
    private Optional<TraceSummary> summarize(Trace trace) {
        return trace.getSpans().stream()
            .filter(s -> s.parentSpanId() == null)
            .findFirst()
            .map(root -> new TraceSummary(
                trace.getTraceId(),
                root.service(),
                root.operation(),
                trace.getSpans().size(),
                (int) ((root.endTime() - root.startTime()) / 1_000_000),
                root.startTime(),
                trace.isHasError() ? "error" : "ok"
            ));
    }

}
