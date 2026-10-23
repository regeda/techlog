package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/regeda/techlog/internal/api"
	"github.com/regeda/techlog/internal/mcpserver"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/ui"
)

func runServe(ctx context.Context, args []string, out io.Writer) (err error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "localhost:8080", "listen address; use :8080 to listen on all interfaces")
	skew := fs.Duration("clock-skew", service.DefaultClockSkew, "how far ahead of the server clock an event time may be")
	sf := addStoreFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := sf.open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()
	svc, err := service.New(st, service.WithClockSkew(*skew))
	if err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(log)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: newHandler(svc, log), ReadHeaderTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if _, err := fmt.Fprintf(out, "techlog listening on http://%s  (UI /, API /api/v1, MCP /mcp; db %s)\n", ln.Addr(), sf.db); err != nil {
		return errors.Join(err, ln.Close())
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown(srv, log)
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// shutdown drains srv for a few seconds, then closes it.
func shutdown(srv *http.Server, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		// an open MCP stream kept the server from draining in time
		if err := srv.Close(); err != nil {
			log.Error("close server", "error", err)
		}
	}
}

// newHandler mounts the three front doors on one mux. Specific routes come
// first; the UI answers everything else (and 404s what it does not own).
func newHandler(svc *service.Service, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", api.Handler(svc))
	mux.Handle("/mcp", mcpserver.Handler(svc))
	mux.Handle("/", ui.Handler(svc))
	return logRequests(log, mux)
}

// statusWriter records the status code. It forwards Flush, which MCP's
// server-sent-event streams need.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "took", time.Since(start).Round(time.Millisecond))
	})
}
