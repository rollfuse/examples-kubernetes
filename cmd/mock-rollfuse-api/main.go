// Command mock-rollfuse-api is a minimal stand-in for the real rollfuse
// platform API, so this example runs end-to-end with `kubectl apply -k
// k8s/` alone — no rollfuse account or credential required. It serves the
// Configuration for GET /v1/config (see config/checkout-config.json,
// mounted from a ConfigMap — k8s/mock-rollfuse-api-configmap.yaml) and
// accepts (and logs) POST /v1/exposure-events, matching the wire shapes
// github.com/rollfuse/go-sdk expects.
//
// Unlike the same fixture in this org's other example repos,
// GET /v1/config re-reads the file on every request rather than caching
// it at startup: the whole point of this example is that editing the
// ConfigMap and re-applying changes the live rollout without restarting
// any checkout Pod — see the README's "Change the rollout, live" section.
//
// This is a demo fixture, not a reference implementation of the rollfuse
// API: it does not validate the Authorization header, version
// Configuration, or persist anything.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	configPath := envOr("CONFIG_PATH", "/config/checkout-config.json")
	listenAddr := envOr("LISTEN_ADDR", ":8090")

	// Fail fast if the fixture isn't even valid JSON at startup — this
	// file is hand-maintained, not generated. Each request re-reads and
	// re-validates it (see the handler below), so a bad edit shows up as
	// a 500 on the next GET /v1/config, not silently ignored.
	if _, err := readValidatedConfig(configPath); err != nil {
		log.Fatalf("%s: %v", configPath, err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/config", func(w http.ResponseWriter, _ *http.Request) {
		configBytes, err := readValidatedConfig(configPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(configBytes)
	})

	mux.HandleFunc("POST /v1/exposure-events", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var batch struct {
			Events []json.RawMessage `json:"events"`
		}
		if err := json.Unmarshal(body, &batch); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)

			return
		}

		log.Printf("exposure-events: accepted %d event(s)", len(batch.Events))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]int{"accepted": len(batch.Events)})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("mock-rollfuse-api: serving %s on %s", configPath, listenAddr)

	if err := http.ListenAndServe(listenAddr, mux); err != nil { //nolint:gosec // demo-only fixture, no external exposure
		log.Fatal(err)
	}
}

// readValidatedConfig reads configPath fresh and confirms it's valid
// JSON, returning an error a caller can turn into a 500 instead of ever
// serving malformed or partially-written content (a ConfigMap volume
// update is not atomic from a reader's perspective mid-write, though
// kubelet's actual mechanism — symlink swap — makes a torn read very
// unlikely in practice).
func readValidatedConfig(path string) ([]byte, error) {
	configBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var probe map[string]any
	if err := json.Unmarshal(configBytes, &probe); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}

	return configBytes, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
