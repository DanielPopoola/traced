package com.traced.uchiha.traced.span;

public record TraceSummary(
    String traceId,
    String rootService,
    String rootOperation,
    int spanCount,
    int durationMs,
    long startTime,
    String status
) {}