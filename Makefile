BINARY := argocd-lint
BUILD_DIR := bin
DIST_DIR := dist
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')

.PHONY: build clean test fmt lint release examples seed check

build:
	@echo "Building $(BINARY)"
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY) ./cmd/argocd-lint

clean:
	@rm -rf $(BUILD_DIR) $(DIST_DIR)

fmt:
	@gofmt -w $(GO_FILES)

check:
	@gofmt -l $(GO_FILES) | tee /tmp/gofmt.out
	@if [ -s /tmp/gofmt.out ]; then echo "Go files need formatting" && exit 1; fi
	@go test ./...

lint:
	@$(BUILD_DIR)/$(BINARY) examples/apps --format table

examples:
	@$(BUILD_DIR)/$(BINARY) examples/apps --format table

release:
	@mkdir -p $(DIST_DIR)
	@for GOOS in linux darwin windows; do \
		for GOARCH in amd64 arm64; do \
			EXT=""; \
			if [ "$$GOOS" = "windows" ]; then EXT=".exe"; fi; \
			OUTPUT="$(DIST_DIR)/$(BINARY)-$$GOOS-$$GOARCH$$EXT"; \
			echo "Building $$OUTPUT"; \
			GOOS=$$GOOS GOARCH=$$GOARCH go build -o $$OUTPUT ./cmd/argocd-lint; \
		done; \
	done

seed:
	@echo "Placeholder for seeding test data"

test:
	@go test ./...
