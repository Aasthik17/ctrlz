BINARY  := ctrlz
DIST    := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/Aasthik17/ctrlz/internal/cli.version=$(VERSION)

PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

.PHONY: build
build:
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		out=$(DIST)/$(BINARY)-$$os-$$arch; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $$out ./cmd/ctrlz || exit 1; \
	done

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	rm -rf $(DIST)
