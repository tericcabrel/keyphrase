// Command server serves the keyword extraction API and the page-text route.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/tericcabrel/keyphrase/internal/httpapi"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	// Each request allocates a few MB against a tiny live heap, so the
	// default GOGC=100 collects about once per request. On one CPU that
	// produced 100-300 ms tail latency under concurrent load; GOGC=400 with
	// a 256 MiB soft limit removes it while staying far inside 512 MB.
	// The standard GOGC and GOMEMLIMIT variables still take precedence.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(400)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(256 << 20)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("server listening on %s", *addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
