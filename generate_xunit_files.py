#!/usr/bin/env python3
"""
Generate a large number of xunit XML files for performance testing of attach.xunit_results.

Usage:
    python3 generate_xunit_files.py [num_files] [tests_per_file] [failure_rate]

Arguments:
    num_files       Number of XML files to generate (default: 100)
    tests_per_file  Number of test cases per file (default: 50)
    failure_rate    Fraction of tests that should fail, 0.0-1.0 (default: 0.1)

Example:
    python3 generate_xunit_files.py 100 50 0.1
    # Generates 100 files with 50 tests each (5000 total), 10% failures
"""

import os
import sys
import random
import time

XUNIT_TEMPLATE = '''<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="{suite_name}" tests="{num_tests}" failures="{num_failures}" errors="0" time="{total_time:.3f}">
{test_cases}
</testsuite>
'''

TESTCASE_PASS_TEMPLATE = '''  <testcase classname="{classname}" name="{name}" time="{time:.3f}"/>'''

TESTCASE_FAIL_TEMPLATE = '''  <testcase classname="{classname}" name="{name}" time="{time:.3f}">
    <failure message="Test failed" type="AssertionError">
Traceback (most recent call last):
  File "test_{module}.py", line {line}, in {name}
    self.assertEqual(expected, actual)
AssertionError: Expected value did not match actual value.
Expected: {expected}
Actual: {actual}
    </failure>
  </testcase>'''


def generate_test_case(module_name: str, test_num: int, should_fail: bool) -> tuple[str, float, bool]:
    """Generate a single test case XML snippet."""
    classname = f"test_{module_name}.Test{module_name.title()}Suite"
    name = f"test_{module_name}_case_{test_num:04d}"
    duration = random.uniform(0.001, 0.5)

    if should_fail:
        xml = TESTCASE_FAIL_TEMPLATE.format(
            classname=classname,
            name=name,
            time=duration,
            module=module_name,
            line=random.randint(10, 200),
            expected=random.randint(1, 100),
            actual=random.randint(1, 100),
        )
        return xml, duration, True
    else:
        xml = TESTCASE_PASS_TEMPLATE.format(
            classname=classname,
            name=name,
            time=duration,
        )
        return xml, duration, False


def generate_xunit_file(file_num: int, tests_per_file: int, failure_rate: float, output_dir: str) -> tuple[int, int]:
    """Generate a single xunit XML file with the specified number of tests."""
    module_name = f"module_{file_num:04d}"
    suite_name = f"test_{module_name}.Test{module_name.title()}Suite"

    test_cases = []
    total_time = 0.0
    num_failures = 0

    for test_num in range(tests_per_file):
        should_fail = random.random() < failure_rate
        xml, duration, failed = generate_test_case(module_name, test_num, should_fail)
        test_cases.append(xml)
        total_time += duration
        if failed:
            num_failures += 1

    xml_content = XUNIT_TEMPLATE.format(
        suite_name=suite_name,
        num_tests=tests_per_file,
        num_failures=num_failures,
        total_time=total_time,
        test_cases="\n".join(test_cases),
    )

    filename = os.path.join(output_dir, f"junit-{file_num:04d}.xml")
    with open(filename, "w") as f:
        f.write(xml_content)

    return tests_per_file, num_failures


def main():
    num_files = int(sys.argv[1]) if len(sys.argv) > 1 else 100
    tests_per_file = int(sys.argv[2]) if len(sys.argv) > 2 else 50
    failure_rate = float(sys.argv[3]) if len(sys.argv) > 3 else 0.1

    # Output to current directory (should be src/ when run from evergreen)
    output_dir = "."

    print(f"Generating {num_files} xunit files with {tests_per_file} tests each...")
    print(f"Total tests: {num_files * tests_per_file}")
    print(f"Expected failures: ~{int(num_files * tests_per_file * failure_rate)}")
    print()

    start_time = time.time()

    total_tests = 0
    total_failures = 0

    for file_num in range(num_files):
        tests, failures = generate_xunit_file(file_num, tests_per_file, failure_rate, output_dir)
        total_tests += tests
        total_failures += failures

        if (file_num + 1) % 10 == 0:
            print(f"  Generated {file_num + 1}/{num_files} files...")

    elapsed = time.time() - start_time

    print()
    print(f"Done! Generated {num_files} files in {elapsed:.2f}s")
    print(f"Total tests: {total_tests}")
    print(f"Total failures: {total_failures}")
    print()
    print("Files will be picked up by attach.xunit_results in post section.")


if __name__ == "__main__":
    main()
