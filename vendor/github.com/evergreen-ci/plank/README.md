# Plank

A [Logkeeper](https:://github.com/evergreen-ci/logkeeper) API client in Go.

## Development
The Plank project uses a `makefile` to coordinate compilation and testing. All output from the make commands is written to the git-ignored directory `build`.

### `compile`
Compiles non-test code.

### `test`, `test-<package>`
Runs all tests or the tests of a specific package, sequentially.
- If you would like to specify tests to run using a regex, set the environment variable `RUN_TEST`.
- If you would like to specify a run count, set the environment variable `RUN_COUNT`.
- If you would like to run the tests using the Go race detector, set the environment variable `RACE_DETECTOR`.

### `lint`, `lint-<package>`
Installs and runs the `golangci-lint` linter.

### `mod-tidy`
Runs `go mod tidy`.

### `verify-mod-tidy`
Verifies that `go mod tidy` has been run to clean up the go.mod and go.sum files.

### `clean`
Removes the `build` directory.

### `clean-results`
Removes all of the output files from the `build` directory.

File tickets in JIRA with the [EVG](https://jira.mongodb.org/projects/EVG) project.
