package observability

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func GinTracing() gin.HandlerFunc {
	tracer := otel.Tracer(ServiceName)
	return func(c *gin.Context) {
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)
		ctx, span := tracer.Start(
			ctx,
			c.Request.Method,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		span.SetName(c.Request.Method + " " + route)
		span.SetAttributes(
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", c.Writer.Status()),
		)
		if c.Writer.Status() >= 500 {
			span.SetStatus(codes.Error, "HTTP server error")
		}
	}
}

func GinMetrics() (gin.HandlerFunc, error) {
	meter := otel.Meter(ServiceName)
	requests, err := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Number of completed HTTP server requests."),
	)
	if err != nil {
		return nil, err
	}
	duration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests."),
	)
	if err != nil {
		return nil, err
	}

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		attrs := metric.WithAttributes(
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", c.Writer.Status()),
		)
		ctx := c.Request.Context()
		requests.Add(ctx, 1, attrs)
		duration.Record(ctx, time.Since(start).Seconds(), attrs)
	}, nil
}
