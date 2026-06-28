package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/moeryomenko/squad"
)

// Simple example empty service.
// Into two expressions you can create simple service with
// healthchecker and signal handler, which will provide
// graceful shutdown service.
func main() {
	s, err := squad.New(squad.WithGracefulPeriod(2 * time.Second))
	if err != nil {
		log.Fatalf("service could not start, reason: %v", err)
	}

	http.HandleFunc(`/echo`, func(w http.ResponseWriter, r *http.Request) {
		log.Printf(`handle request from: %s`, sanitizeLogString(r.Header.Get(`User-Agent`)))

		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf(`read body failed: %s`, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(body); err != nil {
			log.Printf(`write response failed: %s`, err.Error())
		}
	})

	s.RunServer(&http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 30 * time.Second,
	})

	s.RunGracefully(func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, func(_ context.Context) error {
		fmt.Println("service shutdowning...")
		time.Sleep(3 * time.Second)

		// never printed, because we set shutdown timeout 2 seconds.
		fmt.Println("service shutdowned")

		return nil
	})

	if err := s.Wait(); err != nil {
		log.Fatalf("service wait failed: %v", err)
	}
}

// sanitizeLogString removes control characters from a string for safe logging.
func sanitizeLogString(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' {
			return -1
		}
		return r
	}, s)
}
