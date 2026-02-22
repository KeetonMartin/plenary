.PHONY: build build-full test test-full web clean

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

clean:
	rm -f plenary
	rm -rf cmd/plenary/web/dist cmd/plenary/web/node_modules
