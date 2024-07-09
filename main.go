package main

import (
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httptracer"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const (
	ServiceName    = "example"
	ServiceVersion = "v0.1.0"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	// Start the regular tracer and return it as an opentracing.Tracer interface. You
	// may use the same set of options as you normally would with the Datadog tracer.
	t := opentracer.New(
		tracer.WithServiceName(ServiceName),
	)

	// Stop it using the regular Stop call for the tracer package.
	defer tracer.Stop()

	opentracing.SetGlobalTracer(t)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(httptracer.Tracer(t, httptracer.Config{
		ServiceName:    ServiceName,
		ServiceVersion: ServiceVersion,
		SampleRate:     1,
		SkipFunc: func(r *http.Request) bool {
			return r.URL.Path == "/health"
		},
		Tags: map[string]interface{}{
			"_dd.measured": 1,
		},
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		span, ctx := opentracing.StartSpanFromContext(r.Context(), "IndexHandler")
		defer span.Finish()

		// Example of how to use the context with the span
		dbCall, _ := opentracing.StartSpanFromContext(ctx, "DBCall")
		time.Sleep(100 * time.Millisecond)
		dbCall.Finish()

		io.WriteString(w, "Hello, World!")
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})

	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    net.JoinHostPort("", port),
		Handler: r,
	}

	logger.Info("http server listening", zap.String("port", port))
	if err := srv.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			logger.Error("http server error", zap.Error(err))
		}
	}
}
