# start project configuration
name := gimlet
buildDir := build
packages := $(name) acl ldap okta rolemanager usercache
compilePackages := $(subst $(name),,$(subst -,/,$(foreach target,$(packages),./$(target))))
orgPath := github.com/evergreen-ci
projectPath := $(orgPath)/$(name)
# end project configuration

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

.DEFAULT_GOAL := compile

# Ensure the build directory exists, since most targets require it.
$(shell mkdir -p $(buildDir))

# start lint setup targets
lintDeps := $(buildDir)/run-linter $(buildDir)/golangci-lint
$(buildDir)/golangci-lint:
	@curl --retry 10 --retry-max-time 60 -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(buildDir) v1.64.5 >/dev/null 2>&1
$(buildDir)/run-linter: cmd/run-linter/run-linter.go $(buildDir)/golangci-lint
	@$(gobin) build -o $@ $<
# end lint setup targets

# start output files
testOutput := $(subst -,/,$(foreach target,$(packages),$(buildDir)/output.$(target).test))
lintOutput := $(subst -,/,$(foreach target,$(packages),$(buildDir)/output.$(target).lint))
coverageOutput := $(subst -,/,$(foreach target,$(packages),$(buildDir)/output.$(target).coverage))
htmlCoverageOutput := $(subst -,/,$(foreach target,$(packages),$(buildDir)/output.$(target).coverage.html))
.PRECIOUS: $(testOutput) $(lintOuptut) $(coverageOutput) $(htmlCoverageOutput)
# end output files

# start basic development operations
lint: $(lintOutput)
compile:
	$(gobin) build $(compilePackages)
test: $(testOutput)
coverage: $(coverageOutput)
html-coverage: $(htmlCoverageOutput)
phony := compile test lint coverage html-coverage

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
ifneq (,$(RUN_TEST))
testArgs += -run='$(RUN_TEST)'
endif
ifneq (,$(RUN_COUNT))
testArgs += -count=$(RUN_COUNT)
endif
ifneq (,$(TEST_TIMEOUT))
testArgs += -timeout=$(TEST_TIMEOUT)
endif
$(buildDir)/output.%.test: .FORCE
	$(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) | tee $(buildDir)/output.$(subst /,-,$*).test
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

# start mongodb targets
mongodb/.get-mongodb:
	rm -rf mongodb
	mkdir -p mongodb
	cd mongodb && curl "$(MONGODB_URL)" -o mongodb.tgz && $(MONGODB_DECOMPRESS) mongodb.tgz && chmod +x ./mongodb-*/bin/*
	cd mongodb && mv ./mongodb-*/bin/* . && rm -rf db_files && rm -rf db_logs && mkdir -p db_files && mkdir -p db_logs
mongodb/.get-mongosh:
	rm -rf mongosh
	mkdir -p mongosh
	cd mongosh && curl "$(MONGOSH_URL)" -o mongosh.tgz && $(MONGOSH_DECOMPRESS) mongosh.tgz && chmod +x ./mongosh-*/bin/*
	cd mongosh && mv ./mongosh-*/bin/* .
get-mongodb: mongodb/.get-mongodb
	@touch $<
get-mongosh: mongodb/.get-mongosh
	@touch $<
start-mongod: mongodb/.get-mongodb
	./mongodb/mongod --dbpath ./mongodb/db_files --port 27017 --replSet amboy
	@echo "waiting for mongod to start up"
check-mongod: mongodb/.get-mongodb mongodb/.get-mongosh
	./mongosh/mongosh --nodb ./scripts/waitForMongo.js
	@echo "mongod is up"
init-rs: mongodb/.get-mongodb mongodb/.get-mongosh
	./mongosh/mongosh --eval 'rs.initiate()'
phony += get-mongodb get-mongosh start-mongod check-mongod init-rs
# end mongodb targets

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
phony += clean clean-results
# end cleanup targets

# configure phony targets
.FORCE:
.PHONY: $(phony)
