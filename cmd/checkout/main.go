// Command checkout is a minimal e-commerce checkout service demonstrating
// @rollfuse/go-sdk running on Kubernetes: every request evaluates the
// "checkout-redesign" flag locally (no network call on the hot path, per
// ADR 0004 — see the SDK's own README), and logs the outcome as a
// structured line so `kubectl logs` shows the rollout happening in real
// time, without needing a metrics stack.
package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	rollfuse "github.com/rollfuse/go-sdk"
)

const flagKey = "checkout-redesign"

func main() {
	baseURL := envOr("ROLLFUSE_API_BASE_URL", "http://localhost:8090")
	credential := envOr("ROLLFUSE_SERVICE_CREDENTIAL", "demo-credential")
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	podName := envOr("POD_NAME", "local")

	client, err := rollfuse.NewClient(baseURL, credential)
	if err != nil {
		log.Fatalf("rollfuse.NewClient: %v", err)
	}
	defer client.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Start(startCtx); err != nil {
		log.Fatalf("client.Start: %v (is the rollfuse API at %s reachable?)", err, baseURL)
	}

	log.Printf("checkout[%s]: connected to %s, serving on %s", podName, baseURL, listenAddr)

	mux := http.NewServeMux()
	mux.HandleFunc("/checkout", checkoutHandler(client, podName))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		// Ready only once the SDK has a cached Configuration — start()
		// above already blocked on that, so by the time this handler is
		// registered it's always true, but a readiness probe should say
		// so explicitly rather than assume.
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{Addr: listenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

type checkoutResponse struct {
	Pod          string `json:"pod"`
	SubjectKey   string `json:"subject_key"`
	VariationKey string `json:"variation_key"`
	Reason       string `json:"reason"`
}

func checkoutHandler(client *rollfuse.Client, podName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subjectKey := r.URL.Query().Get("user")
		if subjectKey == "" {
			subjectKey = "user_" + strconv.Itoa(rand.IntN(10_000))
		}

		result, err := client.Evaluate(subjectKey, flagKey, rollfuse.WithFallback(false))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		log.Printf("checkout[%s]: subject=%s variation=%s reason=%s", podName, subjectKey, result.VariationKey, result.Reason)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(checkoutResponse{
			Pod:          podName,
			SubjectKey:   subjectKey,
			VariationKey: result.VariationKey,
			Reason:       string(result.Reason),
		})
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
