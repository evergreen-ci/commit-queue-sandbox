package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/evergreen-ci/poplar"
	"github.com/evergreen-ci/poplar/rpc/internal"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
)

// UploadReportOptions contains all the required options for uploading a report
// to both Cedar and SPS.

type UploadReportOptions struct {
	Report          *poplar.Report
	HTTPClient      *http.Client
	SPSURL          string
	SerializeUpload bool
	DryRun          bool
}

// UploadReport does the following:
// 1. Ingest the report.json.
// 2. Upload the report to SPS over a REST endpoint.
func UploadReport(ctx context.Context, opts UploadReportOptions) error {
	if err := opts.convertAndUploadArtifacts(ctx); err != nil {
		return errors.Wrap(err, "uploading tests for report")
	}
	if !opts.DryRun {
		if opts.HTTPClient == nil {
			opts.HTTPClient = utility.GetHTTPClient()
			defer utility.PutHTTPClient(opts.HTTPClient)
		}
		if err := uploadTestsToSPS(ctx, opts.Report, opts.HTTPClient, opts.SPSURL); err != nil {
			return errors.Wrap(err, "uploading metrics for report to SPS")
		}
	}
	return nil
}

func uploadTestsToSPS(ctx context.Context, report *poplar.Report, client *http.Client, spsURL string) error {
	taskInformation := &internal.TaskInformation{
		Project:   report.Project,
		Version:   report.Version,
		Variant:   report.Variant,
		Order:     report.Order,
		TaskName:  report.TaskName,
		TaskId:    report.TaskID,
		Execution: report.Execution,
		Mainline:  report.Mainline,
	}
	perfResults := internal.GatherPerfResults(report)
	requestBody := &internal.SubmitPerfResultRequest{
		Id:      *taskInformation,
		Results: perfResults,
	}
	marshalledBody, err := json.Marshal(requestBody)
	if err != nil {
		return errors.Wrap(err, "marshalling request body")
	}
	bodyReader := bytes.NewReader(marshalledBody)
	requestURL := spsURL + "/raw_perf_results"
	request, err := http.NewRequestWithContext(ctx, "POST", requestURL, bodyReader)
	if err != nil {
		return errors.Wrap(err, "creating request")
	}
	response, err := client.Do(request)
	if err != nil {
		return errors.Wrap(err, "sending request")
	}
	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err == nil {
			return errors.Errorf("unexpected status code: %d: %s", response.StatusCode, body)
		} else {
			return errors.Wrapf(err, "unexpected status code: %d", response.StatusCode)
		}

	}
	return nil
}

func (opts *UploadReportOptions) convertAndUploadArtifacts(ctx context.Context) error {
	catcher := grip.NewBasicCatcher()
	testChan := make(chan poplar.Test, len(opts.Report.Tests)*2)
	go opts.artifactProducer(ctx, testChan, catcher)

	var wg sync.WaitGroup
	workers := 1
	if !opts.SerializeUpload {
		workers = runtime.NumCPU()
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go opts.artifactConsumer(ctx, testChan, catcher, &wg)
	}

	wg.Wait()
	return catcher.Resolve()
}

func (opts *UploadReportOptions) artifactProducer(ctx context.Context, testChan chan poplar.Test, catcher grip.Catcher) {
	defer func() {
		catcher.Add(recovery.HandlePanicWithError(recover(), nil, "artifact upload producer"))
		close(testChan)
	}()

	testQueue := make([]poplar.Test, len(opts.Report.Tests))
	for i, test := range opts.Report.Tests {
		testQueue[i] = test
	}
	for len(testQueue) != 0 {
		test := testQueue[0]
		testQueue = testQueue[1:]
		select {
		case testChan <- test:
		case <-ctx.Done():
			return
		}

		for _, subTest := range test.SubTests {
			testQueue = append(testQueue, subTest)
		}
	}
}

func (opts *UploadReportOptions) artifactConsumer(ctx context.Context, testChan chan poplar.Test, catcher grip.Catcher, wg *sync.WaitGroup) {
	defer func() {
		catcher.Add(recovery.HandlePanicWithError(recover(), nil, "artifact upload consumer"))
		wg.Done()
	}()

	for test := range testChan {
		for j := range test.Artifacts {
			if err := ctx.Err(); err != nil {
				catcher.Add(err)
				return
			}

			if err := test.Artifacts[j].Convert(ctx); err != nil {
				catcher.Wrap(err, "converting artifact")
				continue
			}

			if err := test.Artifacts[j].SetBucketInfo(opts.Report.BucketConf); err != nil {
				catcher.Wrap(err, "setting bucket info")
				continue
			}

			grip.Info(message.Fields{
				"op":     "uploading artifact",
				"path":   test.Artifacts[j].Path,
				"bucket": test.Artifacts[j].Bucket,
				"prefix": test.Artifacts[j].Prefix,
				"file":   test.Artifacts[j].LocalFile,
			})
			catcher.Wrapf(test.Artifacts[j].Upload(ctx, opts.Report.BucketConf, opts.DryRun), "uploading artifact")
		}
	}
}
