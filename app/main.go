// Package main implements a minimal HTTP service used as the CI/CD demo target
// for Chapter 07 (GitOps). It exposes three endpoints:
//
//	GET /         → greeting including the build version
//	GET /healthz  → liveness/readiness probe (always 200 once serving)
//	GET /version  → build version + git commit (injected via -ldflags at build time)
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// These are overridden at build time via:
//
//	go build -ldflags="-X main.version=v1 -X main.commit=$(git rev-parse HEAD)"
var (
	version = "dev"
	commit  = "none"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, "hello from web-app %s\n", version)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "version=%s commit=%s\n", version, commit)
	})

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	log.Printf("web-app %s (commit %s) listening on %s", version, commit, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
