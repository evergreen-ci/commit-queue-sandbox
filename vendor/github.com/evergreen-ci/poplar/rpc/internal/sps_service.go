package internal

import (
	"time"

	"github.com/evergreen-ci/poplar"
)

type TaskInformation struct {
	Project   string `json:"project"`
	Version   string `json:"version"`
	Variant   string `json:"variant"`
	Order     int    `json:"order"`
	TaskName  string `json:"task_name"`
	TaskId    string `json:"task_id"`
	Execution int    `json:"execution"`
	Mainline  bool   `json:"mainline"`
}

type TestInformation struct {
	TestName  string           `json:"test_name"`
	Arguments map[string]int32 `json:"arguments"`
}

type TestMetric struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type PerfResult struct {
	Info        TestInformation `json:"info"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt time.Time       `json:"completed_at"`
	Metrics     []TestMetric    `json:"metrics"`
}

type SubmitPerfResultRequest struct {
	Id      TaskInformation `json:"id"`
	Results []PerfResult    `json:"results"`
}

func GatherPerfResults(report *poplar.Report) []PerfResult {
	tests := gatherTests(report.Tests)
	perfResults := []PerfResult{}
	for _, test := range tests {
		testInformation := TestInformation{
			TestName:  test.Info.TestName,
			Arguments: test.Info.Arguments,
		}
		perfResult := PerfResult{
			Info:        testInformation,
			CreatedAt:   test.CreatedAt,
			CompletedAt: test.CompletedAt,
			Metrics:     []TestMetric{},
		}
		for _, metric := range test.Metrics {
			perfResult.Metrics = append(perfResult.Metrics, TestMetric{
				Name:  metric.Name,
				Value: metric.Value,
			})
		}
		perfResults = append(perfResults, perfResult)
	}
	return perfResults
}

func gatherTests(tests []poplar.Test) []poplar.Test {
	gatheredTests := []poplar.Test{}
	for _, test := range tests {
		gatheredTests = append(gatheredTests, test)
		gatheredTests = append(gatheredTests, gatherTests(test.SubTests)...)
	}
	return gatheredTests
}
