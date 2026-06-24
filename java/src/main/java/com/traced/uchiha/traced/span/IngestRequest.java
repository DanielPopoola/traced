package com.traced.uchiha.traced.span;

import java.util.List;

public record IngestRequest(List<Span> spans) {}