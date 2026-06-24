package com.traced.uchiha.traced.span;

import java.util.Map;

public record Span(
    String traceId,
    String spanId,
    String parentSpanId,
    String service,
    String operation,
    String status,
    long startTime,
    long endTime,
    Map<String, String> tags
) {}