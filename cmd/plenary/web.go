//go:build webembed

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	plenary "github.com/keetonmartin/plenary/internal/plenary"
)

//go:embed web/dist
var webDist embed.FS

func cmdWeb(store *plenary.JSONLStore, args []string) error {
	port, _ := getFlag(args, "--port")
	if port == "" {
		port = "3000"
	}

	mux := http.NewServeMux()

	// API: list all plenaries
	mux.HandleFunc("/api/plenaries", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		events, err := store.ListAll()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Group by plenary_id, reduce each
		grouped := map[string][]plenary.Event{}
		for _, evt := range events {
			grouped[evt.PlenaryID] = append(grouped[evt.PlenaryID], evt)
		}

		type plenarySummary struct {
			PlenaryID   string `json:"plenary_id"`
			Topic       string `json:"topic"`
			Phase       string `json:"phase"`
			Rule        string `json:"decision_rule"`
			Closed      bool   `json:"closed"`
			Events      int    `json:"event_count"`
			LastEventAt string `json:"last_event_at,omitempty"`
		}

		summaries := make([]plenarySummary, 0, len(grouped))
		for pid, evts := range grouped {
			snap, err := plenary.Reduce(evts)
			if err != nil {
				continue
			}
			lastEventAt := ""
			if n := len(evts); n > 0 {
				lastEventAt = evts[n-1].TS
			}
			summaries = append(summaries, plenarySummary{
				PlenaryID:   pid,
				Topic:       snap.Topic,
				Phase:       string(snap.Phase),
				Rule:        string(snap.DecisionRule),
				Closed:      snap.Closed,
				Events:      snap.EventCount,
				LastEventAt: lastEventAt,
			})
		}

		sort.Slice(summaries, func(i, j int) bool {
			if summaries[i].Closed != summaries[j].Closed {
				return !summaries[i].Closed
			}
			if summaries[i].LastEventAt != summaries[j].LastEventAt {
				return summaries[i].LastEventAt > summaries[j].LastEventAt
			}
			return summaries[i].PlenaryID < summaries[j].PlenaryID
		})

		json.NewEncoder(w).Encode(summaries)
	})

	// API: get snapshot for a plenary
	mux.HandleFunc("/api/plenaries/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		path := strings.TrimPrefix(r.URL.Path, "/api/plenaries/")
		parts := strings.SplitN(path, "/", 2)
		pid := parts[0]

		if pid == "" {
			http.Error(w, "plenary_id required", 400)
			return
		}

		events, err := store.ListByPlenary(pid)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if len(events) == 0 {
			http.Error(w, "not found", 404)
			return
		}

		// Check if requesting events
		if len(parts) > 1 && parts[1] == "events" {
			json.NewEncoder(w).Encode(events)
			return
		}

		snap, err := plenary.Reduce(events)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(snap)
	})

	// Serve embedded frontend
	distFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		return fmt.Errorf("failed to load embedded web assets: %w", err)
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// SPA: serve index.html for non-file paths
		if r.URL.Path != "/" && !strings.Contains(r.URL.Path, ".") {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	addr := "127.0.0.1:" + port
	fmt.Printf("Plenary web viewer: http://%s\n", addr)

	// Try to open browser
	go openBrowser("http://" + addr)

	return http.ListenAndServe(addr, mux)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		cmd.Run()
	}
}
