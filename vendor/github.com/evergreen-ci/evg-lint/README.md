evg-lint is a linter for common problems encountered in the 
[Evergreen](https://github.com/evergreen-ci/evergreen) codebase 

## Installation

Golint requires Go 1.6 or later.

    go get -u github.com/evergreen-ci/evg-lint/evg-lint

## Usage

Invoke `evg-lint` exactly as you would `golint`

## Linters

### Testify
Alerts to the use of "Teardown" or "SetUp", which should be "TearDown" and "Setup"
respectively in testify suites. Requires that the receiver struct have
"suite.Suite", or another inline struct with suite.Suite

### Cancelled Spell Check
To aid in grep use, we prefer the AmE spelling "canceled" as opposed to the BrE
spelling "cancelled". This alerts to the use of the BrE spelling. By default,
these errors are not reported. (0.7 confidence; min is 0.8)

### Defer in for loops
Using defer in a for loop has the often unintended consequence of delaying
the defer method until function close, while we mentally expect the defer to
run at the end of every loop body. 

## License
This package derives heavily from [golint](https://github.com/golang/lint),
and remains under the same license
