PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: test build lint clean

test:
	go test ./...

lint:
	go vet ./...

# Static binaries (CGO_ENABLED=0) for each platform in dist/mgmt-<os>-<arch>.
build:
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "build dist/mgmt-$$os-$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" -o dist/mgmt-$$os-$$arch ./cmd/mgmt || exit 1; \
	done

clean:
	rm -rf dist
