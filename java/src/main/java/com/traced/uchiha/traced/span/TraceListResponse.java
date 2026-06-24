package com.traced.uchiha.traced.span;

import java.util.List;

public record TraceListResponse(int total, List<TraceSummary> traces) {}