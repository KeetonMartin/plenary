.PHONY: build test web clean

build: web
	go build -o plenary ./cmd/plenary

web:
	cd cmd/plenary/web && npm install && npm run build

test: web
	go test ./...

clean:
	rm -f plenary
	rm -rf cmd/plenary/web/dist cmd/plenary/web/node_modules
