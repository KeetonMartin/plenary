.PHONY: build build-full test test-full web smoke-http clean

# Build without web viewer (no npm required)
build:
	go build -o plenary ./cmd/plenary

# Build with embedded web viewer
build-full: web
	go build -tags webembed -o plenary ./cmd/plenary

web:
	cd cmd/plenary/web && npm install && npm run build

# Run tests (no npm required)
test:
	go test ./...

# Run tests including web embed
test-full: web
	go test -tags webembed ./...

# Smoke-test two actors over the HTTP API (requires a running `plenary serve`)
smoke-http:
	bash scripts/http_cross_agent_smoke.sh

clean:
	rm -f plenary
	rm -rf cmd/plenary/web/dist cmd/plenary/web/node_modules
