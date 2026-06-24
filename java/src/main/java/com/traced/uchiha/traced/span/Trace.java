package com.traced.uchiha.traced.span;

import java.util.ArrayList;
import java.util.List;

public class Trace {
    private final String traceId;
    private final List<Span> spans = new ArrayList<>();
    private boolean hasError;

    public Trace(String traceId) {
        this.traceId = traceId;
    }

    public String getTraceId() { return traceId; }
    public List<Span> getSpans() { return spans; }
    public boolean isHasError() { return hasError; }

    public void addSpan(Span span) {
        spans.add(span);
        if ("error".equals(span.status())) {
            hasError = true;
        }
    }
}
