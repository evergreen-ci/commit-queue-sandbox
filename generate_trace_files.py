#!/usr/bin/env python3
"""
Generate OTel trace files for performance testing of upload-traces.

Usage:
    python3 generate_trace_files.py [num_files] [spans_per_file]

Arguments:
    num_files       Number of trace files to generate (default: 10)
    spans_per_file  Number of spans per file (default: 100)

Example:
    python3 generate_trace_files.py 10 100
    # Generates 10 trace files with 100 spans each
"""

import json
import os
import random
import sys
import time


def generate_trace_id():
    """Generate a random 32-character hex trace ID."""
    return ''.join(random.choices('0123456789abcdef', k=32))


def generate_span_id():
    """Generate a random 16-character hex span ID."""
    return ''.join(random.choices('0123456789abcdef', k=16))


def generate_span(trace_id: str, parent_span_id: str, span_num: int, start_time_ns: int) -> dict:
    """Generate a single span."""
    span_id = generate_span_id()
    duration_ns = random.randint(1000000, 100000000)  # 1ms to 100ms

    return {
        "traceId": trace_id,
        "spanId": span_id,
        "parentSpanId": parent_span_id,
        "name": f"operation_{span_num:04d}",
        "kind": random.randint(1, 5),
        "startTimeUnixNano": str(start_time_ns),
        "endTimeUnixNano": str(start_time_ns + duration_ns),
        "attributes": [
            {"key": "db.system", "value": {"stringValue": "mongodb"}},
            {"key": "db.operation", "value": {"stringValue": random.choice(["find", "insert", "update", "delete"])}},
            {"key": "db.name", "value": {"stringValue": "test_db"}},
            {"key": "span.num", "value": {"intValue": str(span_num)}},
        ],
        "status": {}
    }


def generate_trace_data(num_spans: int) -> dict:
    """Generate a complete trace data object with multiple spans."""
    trace_id = generate_trace_id()
    base_time_ns = int(time.time() * 1e9)

    spans = []
    parent_span_id = ""

    for i in range(num_spans):
        span = generate_span(trace_id, parent_span_id, i, base_time_ns + (i * 1000000))
        spans.append(span)
        # Randomly decide if next span is a child or sibling
        if random.random() < 0.3:
            parent_span_id = span["spanId"]
        elif random.random() < 0.5:
            parent_span_id = ""

    return {
        "resourceSpans": [{
            "resource": {
                "attributes": [
                    {"key": "service.name", "value": {"stringValue": "test-service"}},
                    {"key": "service.version", "value": {"stringValue": "1.0.0"}},
                    {"key": "host.name", "value": {"stringValue": "test-host"}},
                ]
            },
            "scopeSpans": [{
                "scope": {
                    "name": "test-tracer"
                },
                "spans": spans
            }]
        }]
    }


def generate_trace_file(file_num: int, spans_per_file: int, output_dir: str) -> int:
    """Generate a single trace file with multiple trace data lines."""
    lines = []

    # Generate multiple trace data objects per file (simulating collector output)
    # Real-world data shows ~275 spans per JSON line, so we target that density
    spans_per_object = 275
    num_trace_objects = max(1, spans_per_file // spans_per_object)

    actual_spans = 0
    for _ in range(num_trace_objects):
        trace_data = generate_trace_data(spans_per_object)
        lines.append(json.dumps(trace_data))
        actual_spans += spans_per_object

    filename = os.path.join(output_dir, f"traces_{file_num:04d}.jsonl")
    with open(filename, "w") as f:
        f.write("\n".join(lines) + "\n")

    return actual_spans


def main():
    num_files = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    spans_per_file = int(sys.argv[2]) if len(sys.argv) > 2 else 100

    # Output to OTelTraces directory (where the agent looks for traces)
    # The agent looks at <task_workdir>/build/OTelTraces, not inside src/
    output_dir = "../build/OTelTraces"
    os.makedirs(output_dir, exist_ok=True)

    print(f"Generating {num_files} trace files with ~{spans_per_file} spans each...")
    print(f"Total spans: ~{num_files * spans_per_file}")
    print(f"Output directory: {output_dir}")
    print()

    start_time = time.time()

    total_spans = 0
    for file_num in range(num_files):
        spans = generate_trace_file(file_num, spans_per_file, output_dir)
        total_spans += spans

        if (file_num + 1) % 5 == 0:
            print(f"  Generated {file_num + 1}/{num_files} files...")

    elapsed = time.time() - start_time

    print()
    print(f"Done! Generated {num_files} files in {elapsed:.2f}s")
    print(f"Total spans: {total_spans}")

    # Verify files were created
    actual_files = [f for f in os.listdir(output_dir) if f.endswith('.jsonl')]
    print(f"Files in output directory: {len(actual_files)}")
    print()
    print("Files will be picked up by upload-traces handler automatically.")


if __name__ == "__main__":
    main()
