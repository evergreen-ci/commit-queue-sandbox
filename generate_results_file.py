#!/usr/bin/env python3
"""
Generate a large JSON results file for performance testing of attach.results.

Usage:
    python3 generate_results_file.py [num_tests] [failure_rate] [log_lines]

Arguments:
    num_tests       Number of test results to generate (default: 500)
    failure_rate    Fraction of tests that should fail, 0.0-1.0 (default: 0.1)
    log_lines       Number of lines in each test's log_raw (default: 20)

Example:
    python3 generate_results_file.py 500 0.1 20
    # Generates 500 test results, 10% failures, 20 log lines each
"""

import json
import random
import sys
import time


def generate_log_content(test_name: str, num_lines: int, failed: bool) -> str:
    """Generate realistic log content for a test."""
    lines = []
    lines.append(f"=== Starting test: {test_name} ===")
    lines.append(f"Test configuration loaded at {time.strftime('%Y-%m-%d %H:%M:%S')}")

    for i in range(num_lines - 4):
        if failed and i == num_lines - 6:
            lines.append(f"ERROR: Assertion failed at line {random.randint(50, 200)}")
            lines.append(f"  Expected: {random.randint(1, 100)}")
            lines.append(f"  Actual: {random.randint(1, 100)}")
        else:
            log_type = random.choice(["INFO", "DEBUG", "TRACE"])
            lines.append(f"[{log_type}] Processing step {i+1}: operation completed successfully")

    status = "FAILED" if failed else "PASSED"
    lines.append(f"=== Test {test_name} {status} ===")

    return "\n".join(lines)


def generate_test_result(test_num: int, failure_rate: float, log_lines: int) -> dict:
    """Generate a single test result."""
    module_name = f"module_{test_num // 100:03d}"
    test_name = f"test_{module_name}.Test{module_name.title()}Suite.test_case_{test_num:05d}"

    should_fail = random.random() < failure_rate
    status = "fail" if should_fail else "pass"

    # Generate timestamps (Python time format - seconds since epoch)
    start_time = 1700000000.0 + (test_num * 0.5)  # Stagger start times
    duration = random.uniform(0.1, 2.0)
    end_time = start_time + duration

    log_content = generate_log_content(test_name, log_lines, should_fail)

    return {
        "test_file": test_name,
        "status": status,
        "start": start_time,
        "end": end_time,
        "log_raw": log_content
    }


def main():
    num_tests = int(sys.argv[1]) if len(sys.argv) > 1 else 500
    failure_rate = float(sys.argv[2]) if len(sys.argv) > 2 else 0.1
    log_lines = int(sys.argv[3]) if len(sys.argv) > 3 else 20

    print(f"Generating results file with {num_tests} tests...")
    print(f"Expected failures: ~{int(num_tests * failure_rate)}")
    print(f"Log lines per test: {log_lines}")
    print()

    start_time = time.time()

    results = []
    num_failures = 0

    for test_num in range(num_tests):
        result = generate_test_result(test_num, failure_rate, log_lines)
        results.append(result)
        if result["status"] == "fail":
            num_failures += 1

        if (test_num + 1) % 100 == 0:
            print(f"  Generated {test_num + 1}/{num_tests} results...")

    output = {"results": results}

    output_file = "test_results.json"
    with open(output_file, "w") as f:
        json.dump(output, f, indent=2)

    elapsed = time.time() - start_time
    file_size = len(json.dumps(output)) / 1024 / 1024  # MB

    print()
    print(f"Done! Generated {num_tests} test results in {elapsed:.2f}s")
    print(f"Total failures: {num_failures}")
    print(f"Output file: {output_file} ({file_size:.2f} MB)")
    print()
    print("File will be picked up by attach.results in post section.")


if __name__ == "__main__":
    main()
