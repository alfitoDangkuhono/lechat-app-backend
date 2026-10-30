package observability

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const databaseSystem = "postgresql"

type operationState struct {
	span  trace.Span
	start time.Time
}

func InstrumentGORM(db *gorm.DB, logWriter io.Writer, slowThreshold time.Duration) error {
	meter := otel.Meter(ServiceName)
	tracer := otel.Tracer(ServiceName)

	duration, err := meter.Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of GORM database operations."),
	)
	if err != nil {
		return err
	}
	operations, err := meter.Int64Counter(
		"db.client.operation.count",
		metric.WithDescription("Number of completed GORM database operations."),
	)
	if err != nil {
		return err
	}
	slowQueries, err := meter.Int64Counter(
		"db.client.slow_queries",
		metric.WithDescription("Number of database operations exceeding the slow-query threshold."),
	)
	if err != nil {
		return err
	}
	errorsCounter, err := meter.Int64Counter(
		"db.client.operation.errors",
		metric.WithDescription("Number of failed GORM database operations."),
	)
	if err != nil {
		return err
	}

	slowLogger := log.New(logWriter, "", log.LstdFlags)
	registerOperation := func(
		operation string,
		registerBefore func(string, func(*gorm.DB)) error,
		registerAfter func(string, func(*gorm.DB)) error,
	) error {
		key := "otel:gorm:" + operation
		before := func(tx *gorm.DB) {
			if tx.Statement == nil || tx.Statement.Context == nil {
				return
			}
			ctx, span := tracer.Start(
				tx.Statement.Context,
				"postgres."+operation,
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					attribute.String("db.system", databaseSystem),
					attribute.String("db.operation.name", operation),
				),
			)
			tx.Statement.Context = ctx
			tx.Statement.Settings.Store(key, operationState{span: span, start: time.Now()})
		}
		after := func(tx *gorm.DB) {
			if tx.Statement == nil {
				return
			}
			value, ok := tx.Statement.Settings.LoadAndDelete(key)
			if !ok {
				return
			}
			state := value.(operationState)
			elapsed := time.Since(state.start)
			labels := []attribute.KeyValue{
				attribute.String("db.system", databaseSystem),
				attribute.String("db.operation.name", operation),
			}
			if tx.Statement.Table != "" {
				labels = append(labels, attribute.String("db.collection.name", tx.Statement.Table))
				state.span.SetAttributes(attribute.String("db.collection.name", tx.Statement.Table))
			}
			attrs := metric.WithAttributes(labels...)
			operations.Add(tx.Statement.Context, 1, attrs)
			duration.Record(tx.Statement.Context, elapsed.Seconds(), attrs)

			if tx.Error != nil {
				state.span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", tx.Error)))
				state.span.SetStatus(codes.Error, "database operation failed")
				errorsCounter.Add(tx.Statement.Context, 1, attrs)
			}
			if slowThreshold > 0 && elapsed >= slowThreshold {
				slowQueries.Add(tx.Statement.Context, 1, attrs)
				slowLogger.Printf(
					"SLOW SQL operation=%s table=%s duration=%s threshold=%s",
					operation,
					tx.Statement.Table,
					elapsed,
					slowThreshold,
				)
			}
			state.span.End()
		}

		if err := registerBefore("gorm:"+operation, before); err != nil {
			return fmt.Errorf("register before callback for %s: %w", operation, err)
		}
		if err := registerAfter("gorm:"+operation, after); err != nil {
			return fmt.Errorf("register after callback for %s: %w", operation, err)
		}
		return nil
	}

	if err := registerOperation("query",
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Query().Before(name).Register("observability:before:query", fn)
		},
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Query().After(name).Register("observability:after:query", fn)
		},
	); err != nil {
		return err
	}
	if err := registerOperation("create",
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Create().Before(name).Register("observability:before:create", fn)
		},
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Create().After(name).Register("observability:after:create", fn)
		},
	); err != nil {
		return err
	}
	if err := registerOperation("update",
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Update().Before(name).Register("observability:before:update", fn)
		},
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Update().After(name).Register("observability:after:update", fn)
		},
	); err != nil {
		return err
	}
	if err := registerOperation("delete",
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Delete().Before(name).Register("observability:before:delete", fn)
		},
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Delete().After(name).Register("observability:after:delete", fn)
		},
	); err != nil {
		return err
	}
	if err := registerOperation("row",
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Row().Before(name).Register("observability:before:row", fn)
		},
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Row().After(name).Register("observability:after:row", fn)
		},
	); err != nil {
		return err
	}
	if err := registerOperation("raw",
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Raw().Before(name).Register("observability:before:raw", fn)
		},
		func(name string, fn func(*gorm.DB)) error {
			return db.Callback().Raw().After(name).Register("observability:after:raw", fn)
		},
	); err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get SQL database pool stats: %w", err)
	}
	connectionUsage, err := meter.Int64ObservableGauge(
		"db.client.connections.usage",
		metric.WithDescription("Current number of database connections by state."),
	)
	if err != nil {
		return err
	}
	connectionMax, err := meter.Int64ObservableGauge(
		"db.client.connections.max",
		metric.WithDescription("Maximum number of open database connections."),
	)
	if err != nil {
		return err
	}
	connectionWaitCount, err := meter.Int64ObservableCounter(
		"db.client.connections.wait.count",
		metric.WithDescription("Cumulative number of waits for a database connection."),
	)
	if err != nil {
		return err
	}
	connectionWaitDuration, err := meter.Float64ObservableCounter(
		"db.client.connections.wait.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Cumulative duration spent waiting for database connections."),
	)
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		stats := sqlDB.Stats()
		dbAttributes := metric.WithAttributes(attribute.String("db.system", databaseSystem))
		observer.ObserveInt64(connectionUsage, int64(stats.InUse), metric.WithAttributes(
			attribute.String("db.system", databaseSystem),
			attribute.String("state", "used"),
		))
		observer.ObserveInt64(connectionUsage, int64(stats.Idle), metric.WithAttributes(
			attribute.String("db.system", databaseSystem),
			attribute.String("state", "idle"),
		))
		observer.ObserveInt64(connectionMax, int64(stats.MaxOpenConnections), dbAttributes)
		observer.ObserveInt64(connectionWaitCount, stats.WaitCount, dbAttributes)
		observer.ObserveFloat64(connectionWaitDuration, stats.WaitDuration.Seconds(), dbAttributes)
		return nil
	}, connectionUsage, connectionMax, connectionWaitCount, connectionWaitDuration)
	return err
}
