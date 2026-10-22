// Package telemetry configures OpenTelemetry tracing and metrics for the
// host and its commands. When no endpoint is configured every instrument
// call is a cheap no-op, so instrumentation can stay on all code paths.
package telemetry

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Options configures the telemetry exporters.
type Options struct {
	// ServiceName identifies the exporter's resource.
	ServiceName string
	// Endpoint is the OTLP/HTTP base URL, e.g. http://localhost:4318.
	// Empty disables OTLP export.
	Endpoint string
	// Stdout exports spans and metrics as JSON to stdout, which is the
	// zero-setup development mode.
	Stdout bool
}

// Shutdown releases both providers, flushing pending data.
type Shutdown func(context.Context) error

// Init installs global tracer and meter providers and rebinds the package
// instruments to them. The returned Shutdown flushes and releases; call it
// before exit.
func Init(ctx context.Context, opts Options) (Shutdown, error) {
	name := opts.ServiceName
	if name == "" {
		name = "keyshell"
	}
	shutdown := func(context.Context) error { return nil }

	if !opts.Stdout && opts.Endpoint == "" {
		// Leave the global noop providers in place.
		return shutdown, nil
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(name),
	)

	var (
		traceExp  sdktrace.SpanExporter
		metricExp sdkmetric.Exporter
		err       error
	)
	if opts.Stdout {
		traceExp, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return shutdown, err
		}
		metricExp, err = stdoutmetric.New()
		if err != nil {
			return shutdown, err
		}
	} else {
		traceExp, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(opts.Endpoint))
		if err != nil {
			return shutdown, err
		}
		metricExp, err = otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(opts.Endpoint))
		if err != nil {
			return shutdown, err
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(15*time.Second))),
	)
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	if _, err := tryBind(mp.Meter(name)); err != nil {
		return shutdown, err
	}
	if err := registerActivePluginGauge(mp.Meter(name)); err != nil {
		return shutdown, err
	}

	flushed := false
	return func(ctx context.Context) error {
		if flushed {
			return nil
		}
		flushed = true
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}, nil
}

// Attrs are the shared attribute keys used across instruments.
var Attrs = struct {
	Mode     string
	Backend  string
	Instance string
	Verb     string
	Kind     string
	Outcome  string
}{
	Mode:     "keyshell.mode",
	Backend:  "keyshell.backend",
	Instance: "keyshell.plugin.instance",
	Verb:     "keyshell.verb",
	Kind:     "keyshell.kind",
	Outcome:  "keyshell.outcome",
}

// inst holds the process-wide instruments. They start bound to the global
// noop provider and are rebound by Init when real exporters are configured.
var inst instruments

type instruments struct {
	queryDuration        metric.Float64Histogram
	backendQueryDuration metric.Float64Histogram
	queryResults         metric.Int64Histogram
	pluginLoadDuration   metric.Float64Histogram
	pluginStarts         metric.Int64Counter
	pluginRestarts       metric.Int64Counter
	inputEvents          metric.Int64Counter
	ipcVerbs             metric.Int64Counter
	frameDuration        metric.Float64Histogram
	resolveDuration      metric.Float64Histogram
	sceneReloads         metric.Int64Counter
}

func init() {
	inst, _ = tryBind(otel.GetMeterProvider().Meter("keyshell"))
}

// bind binds every instrument to m or fails wholesale, so a bad instrument
// name can never leave a nil instrument behind for call sites.
func tryBind(m metric.Meter) (instruments, error) {
	i := instruments{}
	var err error
	if i.queryDuration, err = m.Float64Histogram("keyshell.query.duration",
		metric.WithDescription("Duration of a full launcher query"),
		metric.WithUnit("s")); err != nil {
		return i, err
	}
	if i.backendQueryDuration, err = m.Float64Histogram("keyshell.backend.query.duration",
		metric.WithDescription("Duration of one backend query"),
		metric.WithUnit("s")); err != nil {
		return i, err
	}
	if i.queryResults, err = m.Int64Histogram("keyshell.query.results",
		metric.WithDescription("Number of results returned by a query"),
		metric.WithUnit("{result}")); err != nil {
		return i, err
	}
	if i.pluginLoadDuration, err = m.Float64Histogram("keyshell.plugin.load.duration",
		metric.WithDescription("Duration of a plugin process start and handshake"),
		metric.WithUnit("s")); err != nil {
		return i, err
	}
	if i.pluginStarts, err = m.Int64Counter("keyshell.plugin.starts",
		metric.WithDescription("Number of plugin process starts")); err != nil {
		return i, err
	}
	if i.pluginRestarts, err = m.Int64Counter("keyshell.plugin.restarts",
		metric.WithDescription("Number of plugin processes found dead after starting")); err != nil {
		return i, err
	}
	if i.inputEvents, err = m.Int64Counter("keyshell.input.events",
		metric.WithDescription("Number of input events handled")); err != nil {
		return i, err
	}
	if i.ipcVerbs, err = m.Int64Counter("keyshell.ipc.verbs",
		metric.WithDescription("Number of IPC verbs dispatched")); err != nil {
		return i, err
	}
	if i.frameDuration, err = m.Float64Histogram("keyshell.render.frame.duration",
		metric.WithDescription("Duration of one overlay frame draw"),
		metric.WithUnit("s")); err != nil {
		return i, err
	}
	if i.resolveDuration, err = m.Float64Histogram("keyshell.ksl.resolve.duration",
		metric.WithDescription("Duration of one KSL scene resolve"),
		metric.WithUnit("s")); err != nil {
		return i, err
	}
	if i.sceneReloads, err = m.Int64Counter("keyshell.ksl.scene.reloads",
		metric.WithDescription("Number of KSL scene reloads")); err != nil {
		return i, err
	}
	return i, nil
}

// QueryDuration measures a full launcher query.
func QueryDuration() metric.Float64Histogram { return inst.queryDuration }

// BackendQueryDuration measures one backend query.
func BackendQueryDuration() metric.Float64Histogram { return inst.backendQueryDuration }

// QueryResults counts results returned by a query.
func QueryResults() metric.Int64Histogram { return inst.queryResults }

// PluginLoadDuration measures a plugin process start and handshake.
func PluginLoadDuration() metric.Float64Histogram { return inst.pluginLoadDuration }

// PluginStarts counts plugin process starts.
func PluginStarts() metric.Int64Counter { return inst.pluginStarts }

// PluginRestarts counts plugin processes found dead after starting.
func PluginRestarts() metric.Int64Counter { return inst.pluginRestarts }

// PluginCountFunc, when set before Init, is sampled for the active-plugin
// gauge on the configured providers.
var PluginCountFunc func() int64

// registerActivePluginGauge wires the plugin-count gauge onto the meter.
func registerActivePluginGauge(m metric.Meter) error {
	_, err := m.Int64ObservableGauge("keyshell.plugin.active",
		metric.WithDescription("Number of plugin processes currently running"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			if PluginCountFunc != nil {
				o.Observe(PluginCountFunc())
			}
			return nil
		}))
	return err
}

// InputEvents counts input events handled.
func InputEvents() metric.Int64Counter { return inst.inputEvents }

// IPCVerbs counts IPC verbs dispatched.
func IPCVerbs() metric.Int64Counter { return inst.ipcVerbs }

// FrameDuration measures one overlay frame draw.
func FrameDuration() metric.Float64Histogram { return inst.frameDuration }

// ResolveDuration measures one KSL scene resolve.
func ResolveDuration() metric.Float64Histogram { return inst.resolveDuration }

// SceneReloads counts KSL scene reloads.
func SceneReloads() metric.Int64Counter { return inst.sceneReloads }

// StartSpan starts a span on keyshell's default tracer. It keeps call
// sites terse.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, opts...)
}

// tracer is the default tracer for keyshell spans.
var tracer = otel.Tracer("github.com/darkliquid/keyshell")
