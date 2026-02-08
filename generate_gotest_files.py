#!/usr/bin/env python3
"""
Generate go test output files for performance testing of gotest.parse_files.

Usage:
    python3 generate_gotest_files.py [num_files] [tests_per_file] [failure_rate]

Arguments:
    num_files       Number of .suite files to generate (default: 10)
    tests_per_file  Number of test cases per file (default: 100)
    failure_rate    Fraction of tests that should fail, 0.0-1.0 (default: 0.1)

Example:
    python3 generate_gotest_files.py 10 100 0.1
    # Generates 10 files with 100 tests each (1000 total), 10% failures
"""

import os
import random
import sys
import time


def generate_test_output(test_num: int, module_name: str, should_fail: bool) -> tuple[str, str]:
    """Generate go test output for a single test."""
    test_name = f"Test{module_name.title()}_Case{test_num:04d}"
    duration = random.uniform(0.001, 0.5)

    lines = []
    lines.append(f"=== RUN   {test_name}")

    # Add some log output
    for i in range(random.randint(2, 8)):
        lines.append(f"    {test_name}: log line {i+1}: processing step {i+1}")

    if should_fail:
        lines.append(f"    {test_name}: assertion failed")
        lines.append(f"        Expected: {random.randint(1, 100)}")
        lines.append(f"        Actual: {random.randint(1, 100)}")
        status = "FAIL"
    else:
        status = "PASS"

    lines.append(f"--- {status}: {test_name} ({duration:.3f}s)")

    return "\n".join(lines), status


def generate_suite_file(file_num: int, tests_per_file: int, failure_rate: float, output_dir: str) -> tuple[int, int]:
    """Generate a single go test .suite file."""
    module_name = f"module_{file_num:04d}"

    lines = []
    num_failures = 0
    num_passes = 0

    for test_num in range(tests_per_file):
        should_fail = random.random() < failure_rate
        output, status = generate_test_output(test_num, module_name, should_fail)
        lines.append(output)
        if status == "FAIL":
            num_failures += 1
        else:
            num_passes += 1

    # Add summary line
    if num_failures > 0:
        lines.append(f"FAIL")
    else:
        lines.append(f"PASS")
    lines.append(f"ok  \tgithub.com/test/{module_name}\t{random.uniform(0.5, 5.0):.3f}s")

    filename = os.path.join(output_dir, f"{module_name}.suite")
    with open(filename, "w") as f:
        f.write("\n".join(lines))

    return tests_per_file, num_failures


def main():
    num_files = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    tests_per_file = int(sys.argv[2]) if len(sys.argv) > 2 else 100
    failure_rate = float(sys.argv[3]) if len(sys.argv) > 3 else 0.1

    output_dir = "."

    print(f"Generating {num_files} go test suite files with {tests_per_file} tests each...")
    print(f"Total tests: {num_files * tests_per_file}")
    print(f"Expected failures: ~{int(num_files * tests_per_file * failure_rate)}")
    print()

    start_time = time.time()

    total_tests = 0
    total_failures = 0

    for file_num in range(num_files):
        tests, failures = generate_suite_file(file_num, tests_per_file, failure_rate, output_dir)
        total_tests += tests
        total_failures += failures

        if (file_num + 1) % 5 == 0:
            print(f"  Generated {file_num + 1}/{num_files} files...")

    elapsed = time.time() - start_time

    print()
    print(f"Done! Generated {num_files} files in {elapsed:.2f}s")
    print(f"Total tests: {total_tests}")
    print(f"Total failures: {total_failures}")
    print()
    print("Files will be picked up by gotest.parse_files in post section.")


if __name__ == "__main__":
    main()
