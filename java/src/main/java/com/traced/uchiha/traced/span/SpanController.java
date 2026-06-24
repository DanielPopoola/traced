package com.traced.uchiha.traced.span;

import java.util.List;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;


@RestController
public class SpanController {

    private final TraceStore store;

    public SpanController(TraceStore store) {
        this.store = store;
    }

    @PostMapping("/spans")
    public ResponseEntity<Void> ingestSpans(@RequestBody IngestRequest request) {
        if (request.spans() == null || request.spans().isEmpty()) {
            return ResponseEntity.badRequest().build();
        }
        store.addSpans(request.spans());
        return ResponseEntity.status(HttpStatus.ACCEPTED).build();
    }
    
    @GetMapping("/traces") 
    public ResponseEntity<TraceListResponse> getAllTraces(
            @RequestParam(defaultValue = "0") long after,
            @RequestParam(defaultValue = "0") long before,
            @RequestParam(defaultValue = "20") int limit) {

        if (after != 0 && before !=0 && after >= before) {
            return ResponseEntity.badRequest().build();
        }
        if (limit < 1 || limit > 1000) {
            return ResponseEntity.badRequest().build();
        }

        List<TraceSummary> results = store.getAllTraces(after, before);
        int total = results.size();
        List<TraceSummary> page = results.size() > limit ? results.subList(limit, total) : results;

        return ResponseEntity.ok(new TraceListResponse(total, page));
    }

    @GetMapping("/traces/{id}")
    public ResponseEntity<Trace> getTrace(@PathVariable String id) {
        Trace trace = store.getTrace(id);
        if (trace == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok(trace);
    }

    @GetMapping("/health")
    public ResponseEntity<Void> health() {
        return ResponseEntity.ok().build();
    }
}
