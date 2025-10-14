name := poplar
buildDir := build
pbDir := rpc/internal
packages := $(name) rpc rpc-internal
projectPath := github.com/evergreen-ci/$(name)

# start environment setup
gobin := go
ifneq (,$(GOROOT))
gobin := $(GOROOT)/bin/go
endif

goCache := $(GOCACHE)
ifeq (,$(goCache))
goCache := $(abspath $(buildDir)/.cache)
endif
goModCache := $(GOMODCACHE)
ifeq (,$(goModCache))
goModCache := $(abspath $(buildDir)/.mod-cache)
endif
lintCache := $(GOLANGCI_LINT_CACHE)
ifeq (,$(lintCache))
lintCache := $(abspath $(buildDir)/.lint-cache)
endif

ifeq ($(OS),Windows_NT)
gobin := $(shell cygpath $(gobin))
goCache := $(shell cygpath -m $(goCache))
goModCache := $(shell cygpath -m $(goModCache))
lintCache := $(shell cygpath -m $(lintCache))
export GOROOT := $(shell cygpath -m $(GOROOT))
endif

ifneq ($(goCache),$(GOCACHE))
export GOCACHE := $(goCache)
endif
ifneq ($(goModCache),$(GOMODCACHE))
export GOMODCACHE := $(goModCache)
endif
ifneq ($(lintCache),$(GOLANGCI_LINT_CACHE))
export GOLANGCI_LINT_CACHE := $(lintCache)
endif

ifneq (,$(RACE_DETECTOR))
# cgo is required for using the race detector.
export CGO_ENABLED := 1
else
export CGO_ENABLED := 0
endif
# end environment setup

# Ensure the build directory exists, since most targets require it.
$(shell mkdir -p $(buildDir))

.DEFAULT_GOAL := compile

# start lint setup targets
$(buildDir)/golangci-lint:
	@curl --retry 10 --retry-max-time 60 -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(buildDir) v1.64.5 >/dev/null 2>&1
$(buildDir)/run-linter:cmd/run-linter/run-linter.go $(buildDir)/golangci-lint
	@$(gobin) build -o $@ $<
# end lint setup targets

# start output files
coverageOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage)
htmlCoverageOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage.html)
lintOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).lint)
testOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).test)
.PRECIOUS:$(coverageOutput) $(htmlCoverageOutput) $(lintOutput) $(testOutput)
# end output files

# start basic development operations
compile:
	$(gobin) build $(subst $(name),,$(subst -,/,$(foreach target,$(packages),./$(target))))
test: $(testOutput)
benchPattern := ./
benchmark:
	$(gobin) test $(testArgs) -bench=$(benchPattern) $(if $(RUN_TEST),, -run=^^$$) | tee $(buildDir)/bench.out
coverage: $(coverageOutput)
html-coverage: $(htmlCoverageOutput)
lint: $(lintOutput)

protocVersion := 3.19.3
protoOS := $(shell uname -s | tr A-Z a-z)
ifeq ($(protoOS),darwin)
protoOS := osx
endif
protoOS := $(protoOS)-$(shell uname -m | tr A-Z a-z)
$(buildDir)/protoc:
	curl --retry 10 --retry-max-time 60 -L0 https://github.com/protocolbuffers/protobuf/releases/download/v$(protocVersion)/protoc-$(protocVersion)-$(protoOS).zip --output protoc.zip
	unzip -q protoc.zip -d $(buildDir)/protoc
	rm -f protoc.zip
	GOBIN="$(abspath $(buildDir))" $(gobin) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	GOBIN="$(abspath $(buildDir))" $(gobin) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
proto: $(buildDir)/protoc
	PATH="$(abspath $(buildDir)):$(PATH)" $(buildDir)/protoc/bin/protoc --go_out=. --go-grpc_out=. *.proto
phony += compile lint test coverage html-coverage proto

# start convenience targets for running tests and coverage tasks on a
# specific package.
test-%: $(buildDir)/output.%.test
	
coverage-%: $(buildDir)/output.%.coverage
	
html-coverage-%: $(buildDir)/output.%.coverage.html
	
lint-%: $(buildDir)/output.%.lint
	
# end convenience targets
# end basic development operations

# start test and coverage artifacts
testArgs := -v
ifneq (,$(RACE_DETECTOR))
testArgs += -race
endif
ifneq (,$(RUN_COUNT))
testArgs += -count=$(RUN_COUNT)
endif
ifneq (,$(RUN_TEST))
testArgs += -run='$(RUN_TEST)'
endif
ifneq (,$(SKIP_LONG))
testArgs += -short
endif
$(buildDir)/output.%.test: .FORCE
	$(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) | tee $@
	@grep -s -q -e "^PASS" $@
$(buildDir)/output.%.coverage: .FORCE
	$(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) -covermode=count -coverprofile $@ | tee $(buildDir)/output.$*.test
	@-[ -f $@ ] && $(gobin) tool cover -func=$@ | sed 's%$(projectPath)/%%' | column -t
	@grep -s -q -e "^PASS" $(subst coverage,test,$@)
$(buildDir)/output.%.coverage.html: $(buildDir)/output.%.coverage .FORCE
	$(gobin) tool cover -html=$< -o $@

ifneq (go,$(gobin))
# We have to handle the PATH specially for linting in CI, because if the PATH has a different version of the Go
# binary in it, the linter won't work properly.
lintEnvVars := PATH="$(shell dirname $(gobin)):$(PATH)"
endif
$(buildDir)/output.%.lint: $(buildDir)/run-linter .FORCE
	@$(lintEnvVars) ./$< --output=$@ --lintBin=$(buildDir)/golangci-lint --packages='$*'
# end test and coverage artifacts

# start module management targets
mod-tidy:
	$(gobin) mod tidy
# Check if go.mod and go.sum are clean. If they're clean, then mod tidy should not produce a different result.
verify-mod-tidy:
	$(gobin) run cmd/verify-mod-tidy/verify-mod-tidy.go -goBin="$(gobin)"
phony += mod-tidy verify-mod-tidy
# end module management targets

# start cleanup targets
clean:
	rm -rf $(buildDir)
clean-results:
	rm -rf $(buildDir)/output.*
clean-proto:
	rm -rf $(buildDir)/protoc $(buildDir)/protoc-gen-go $(buildDir)/protoc-gen-go-grpc
phony += clean clean-results clean-proto
# end cleanup targets

# configure phony targets
.FORCE:
.PHONY: $(phony) .FORCE
