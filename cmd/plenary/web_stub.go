//go:build !webembed

package main

import (
	"fmt"

	plenary "github.com/keetonmartin/plenary/internal/plenary"
)

func cmdWeb(_ *plenary.JSONLStore, _ []string) error {
	return fmt.Errorf("web viewer not available: binary was built without web assets. Run 'cd cmd/plenary/web && npm install && npm run build' then rebuild with 'go build -tags webembed'")
}
