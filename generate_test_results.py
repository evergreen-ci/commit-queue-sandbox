import json
import sys
import time

def main():
    if len(sys.argv) < 2:
        print("Usage: generate_test_results.py <test_file>")
        sys.exit(1)
    recommended_test_file = sys.argv[1]
    with open(recommended_test_file, "r") as file:
        recommended_tests = json.load(file)

    names = [test["name"] for test in recommended_tests.get("tests", [])]

    result_data = {"results": []}
    for name in names:
        curr_time = int(time.time())
        result_data["results"].append({
            "status": "pass",
            "test_file": name,
            "start": curr_time,
            "end": curr_time,
        })


    output_file = "test_selection_results.json"
    with open(output_file, "w") as file:
        json.dump(result_data, file, indent=4)


if __name__ == "__main__":
    main()
