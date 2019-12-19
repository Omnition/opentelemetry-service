# More exclusions can be added similar with: -not -path './testbed/*'
ALL_SRC := $(shell find . -name '*.go' \
                                -not -path './vendor/*' \
                                -not -path './tools/*' \
                                -not -path './testbed/*' \
                                -not -path '*/gen/*' \
                                -type f | sort)

# All source code and documents. Used in spell check.
ALL_SRC_AND_DOC := $(shell find . \
                                -name "*.md" -not -path './vendor/*' -o \
								-name "*.go" -not -path './vendor/*' -o \
								-name "*.yaml" -not -path './vendor/*'  \
                                -type f | sort)

# ALL_PKGS is used with 'go cover'
ALL_PKGS := $(shell go list $(sort $(dir $(ALL_SRC))))

GOTEST_OPT?= -race -timeout 30s
GOTEST_OPT_WITH_COVERAGE = $(GOTEST_OPT) -coverprofile=coverage.txt -covermode=atomic
GOTEST=go test
GOFMT=gofmt
GOIMPORTS=goimports
GOLINT=golint
GOMOD?= -mod=vendor
GOVET=go vet
GOOS=$(shell go env GOOS)
ADDLICENCESE= addlicense
MISSPELL=misspell -error
MISSPELL_CORRECTION=misspell -w
STATICCHECK=staticcheck

GIT_SHA=$(shell git rev-parse --short HEAD)
BUILD_INFO_IMPORT_PATH=github.com/Omnition/omnition-opentelemetry-collector/internal/version
BUILD_X1=-X $(BUILD_INFO_IMPORT_PATH).GitHash=$(GIT_SHA)
ifdef VERSION
BUILD_X2=-X $(BUILD_INFO_IMPORT_PATH).Version=$(VERSION)
endif
BUILD_INFO=-ldflags "${BUILD_X1} ${BUILD_X2}"

all-pkgs:
	@echo $(ALL_PKGS) | tr ' ' '\n' | sort

all-srcs:
	@echo $(ALL_SRC) | tr ' ' '\n' | sort

.DEFAULT_GOAL := addlicense-fmt-vet-lint-goimports-misspell-staticcheck-test

.PHONY: addlicense-fmt-vet-lint-goimports-misspell-staticcheck-test
addlicense-fmt-vet-lint-goimports-misspell-staticcheck-test: addlicense fmt vet lint goimports misspell staticcheck test

.PHONY: e2e-test
e2e-test: omnitelsvc
	$(MAKE) -C testbed runtests

.PHONY: test
test:
	$(GOTEST) $(GOMOD) $(GOTEST_OPT) $(ALL_PKGS)

.PHONY: travis-ci
travis-ci: fmt vet lint goimports misspell staticcheck test-with-cover omnitelsvc
	$(MAKE) -C testbed install-tools
	$(MAKE) -C testbed runtests

.PHONY: test-with-cover
test-with-cover:
	@echo Verifying that all packages have test files to count in coverage
	@scripts/check-test-files.sh $(subst github.com/Omnition/omnition-opentelemetry-collector/,./,$(ALL_PKGS))
	@echo pre-compiling tests
	@time go test $(GOMOD) -i $(ALL_PKGS)
	$(GOTEST) $(GOMOD) $(GOTEST_OPT_WITH_COVERAGE) $(ALL_PKGS)
	go tool cover -html=coverage.txt -o coverage.html

.PHONY: addlicense
addlicense:
	@ADDLICENCESEOUT=`$(ADDLICENCESE) -y 2019 -c 'OpenTelemetry Authors' $(ALL_SRC) 2>&1`; \
		if [ "$$ADDLICENCESEOUT" ]; then \
			echo "$(ADDLICENCESE) FAILED => add License errors:\n"; \
			echo "$$ADDLICENCESEOUT\n"; \
			exit 1; \
		else \
			echo "Add License finished successfully"; \
		fi

.PHONY: fmt
fmt:
	@FMTOUT=`$(GOFMT) -s -l $(ALL_SRC) 2>&1`; \
	if [ "$$FMTOUT" ]; then \
		echo "$(GOFMT) FAILED => gofmt the following files:\n"; \
		echo "$$FMTOUT\n"; \
		exit 1; \
	else \
	    echo "Fmt finished successfully"; \
	fi

.PHONY: lint
lint:
	@LINTOUT=`$(GOLINT) $(ALL_PKGS) 2>&1`; \
	if [ "$$LINTOUT" ]; then \
		echo "$(GOLINT) FAILED => clean the following lint errors:\n"; \
		echo "$$LINTOUT\n"; \
		exit 1; \
	else \
	    echo "Lint finished successfully"; \
	fi

.PHONY: goimports
goimports:
	@IMPORTSOUT=`$(GOIMPORTS) -d $(ALL_SRC) 2>&1`; \
	if [ "$$IMPORTSOUT" ]; then \
		echo "$(GOIMPORTS) FAILED => fix the following goimports errors:\n"; \
		echo "$$IMPORTSOUT\n"; \
		exit 1; \
	else \
	    echo "Goimports finished successfully"; \
	fi

.PHONY: misspell
misspell:
	$(MISSPELL) $(ALL_SRC_AND_DOC)

.PHONY: misspell-correction
misspell-correction:
	$(MISSPELL_CORRECTION) $(ALL_SRC_AND_DOC)

.PHONY: staticcheck
staticcheck:
	GOFLAGS=$(GOMOD) $(STATICCHECK) ./...

.PHONY: vet
vet:
	@$(GOVET) $(GOMOD) ./...
	@echo "Vet finished successfully"

.PHONY: install-tools
install-tools:
	GO111MODULE=on go install \
	  github.com/google/addlicense \
	  golang.org/x/lint/golint \
	  golang.org/x/tools/cmd/goimports \
	  github.com/client9/misspell/cmd/misspell \
	  github.com/jstemmer/go-junit-report \
	  github.com/omnition/gogoproto-rewriter \
	  honnef.co/go/tools/cmd/staticcheck
	$(MAKE) dep

.PHONY: dep
dep:
    # go mod vendor doesn't forcibly update folders leading to stale code being
    # present locally. Deleting the vendor directory ensures only the desired
    # version of the code is available.
	rm -fr vendor
	go mod vendor
	gogoproto-rewriter vendor/github.com/open-telemetry/opentelemetry-collector/
	gogoproto-rewriter vendor/github.com/open-telemetry/opentelemetry-collector-contrib/
	gogoproto-rewriter vendor/contrib.go.opencensus.io/exporter/

.PHONY: omnitelsvc
omnitelsvc:
	GO111MODULE=on CGO_ENABLED=0 go build $(GOMOD) -o ./bin/$(GOOS)/omnitelsvc $(BUILD_INFO) ./cmd/omnitelsvc

.PHONY: run
run:
	GO111MODULE=on go run $(GOMOD) --race ./cmd/omnitelsvc --config $(RUN_CONFIG)

.PHONY: docker-component # Not intended to be used directly
docker-component: check-component
	GOOS=linux $(MAKE) $(COMPONENT)
	cp ./bin/linux/$(COMPONENT) ./cmd/$(COMPONENT)/
	docker build -t $(COMPONENT) ./cmd/$(COMPONENT)/
	rm ./cmd/$(COMPONENT)/$(COMPONENT)

.PHONY: check-component
check-component:
ifndef COMPONENT
	$(error COMPONENT variable was not defined)
endif

.PHONY: docker-omnitelsvc
docker-omnitelsvc:
	COMPONENT=omnitelsvc $(MAKE) docker-component

.PHONY: binaries
binaries: omnitelsvc

.PHONY: binaries-all-sys
binaries-all-sys:
	GOOS=darwin $(MAKE) binaries
	GOOS=linux $(MAKE) binaries
	GOOS=windows $(MAKE) binaries

# Helper target to generate Protobuf implementations based on .proto files.
PROTO_PACKAGE_PATH?=./exporter/omnishard/gen/

.PHONY: generate-protobuf
generate-protobuf:
	docker run --rm -v $(PWD):$(PWD) -w $(PWD) \
	    --user $(shell id -u):$(shell id -g) \
	    znly/protoc \
		--go_out=plugins=grpc:$(PROTO_PACKAGE_PATH) \
		-I./exporter/omnishard/ ./exporter/omnishard/*.proto \
