package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"

	"github.com/uptrace/uptrace-go/uptrace"
)

var meter = global.MeterProvider().Meter("app_or_package_name")

func main() {
	ctx := context.Background()

	// Configure OpenTelemetry with sensible defaults.
	uptrace.ConfigureOpentelemetry(
		// copy your project DSN here or use UPTRACE_DSN env var
		// uptrace.WithDSN("https://<key>@api.uptrace.dev/<project_id>"),

		uptrace.WithServiceName("myservice"),
		uptrace.WithServiceVersion("1.0.0"),
	)
	// Send buffered spans and free resources.
	defer uptrace.Shutdown(ctx)

	// Synchronous instruments.
	go counter(ctx)
	go upDownCounter(ctx)
	go histogram(ctx)

	// Asynchronous instruments.
	go counterObserver(ctx)
	go upDownCounterObserver(ctx)
	go gaugeObserver(ctx)

	// Advanced.
	go counterWithLabels(ctx)
	go counterObserverAdvanced()

	fmt.Println("reporting measurements to Uptrace... (press Ctrl+C to stop)")

	ch := make(chan os.Signal, 3)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	<-ch
}

// counter demonstrates how to measure non-decreasing numbers, for example,
// number of requests or connections.
func counter(ctx context.Context) {
	counter, _ := meter.SyncInt64().Counter(
		"some.prefix.counter",
		instrument.WithUnit("1"),
		instrument.WithDescription("TODO"),
	)

	for {
		counter.Add(ctx, 1)
		time.Sleep(time.Millisecond)
	}
}

// upDownCounter demonstrates how to measure numbers that can go up and down, for example,
// number of goroutines or customers.
func upDownCounter(ctx context.Context) {
	counter, _ := meter.SyncInt64().UpDownCounter(
		"some.prefix.up_down_counter",
		instrument.WithUnit("1"),
		instrument.WithDescription("TODO"),
	)

	for {
		if rand.Float64() >= 0.5 {
			counter.Add(ctx, +1)
		} else {
			counter.Add(ctx, -1)
		}

		time.Sleep(time.Second)
	}
}

// histogram demonstrates how to record a distribution of individual values, for example,
// request or query timings. With this instrument you get total number of records,
// avg/min/max values, and heatmaps/percentiles.
func histogram(ctx context.Context) {
	durRecorder, _ := meter.SyncInt64().Histogram(
		"some.prefix.histogram",
		instrument.WithUnit("microseconds"),
		instrument.WithDescription("TODO"),
	)

	for {
		dur := time.Duration(rand.NormFloat64()*5000000) * time.Microsecond
		durRecorder.Record(ctx, dur.Microseconds())

		time.Sleep(time.Millisecond)
	}
}

// counterObserver demonstrates how to measure monotonic (non-decreasing) numbers,
// for example, number of requests or connections.
func counterObserver(ctx context.Context) {
	counter, _ := meter.AsyncInt64().Counter(
		"some.prefix.counter_observer",
		instrument.WithUnit("1"),
		instrument.WithDescription("TODO"),
	)

	var number int64
	if err := meter.RegisterCallback(
		[]instrument.Asynchronous{
			counter,
		},
		// SDK periodically calls this function to collect data.
		func(ctx context.Context) {
			number++
			counter.Observe(ctx, number)
		},
	); err != nil {
		panic(err)
	}
}

// upDownCounterObserver demonstrates how to measure numbers that can go up and down,
// for example, number of goroutines or customers.
func upDownCounterObserver(ctx context.Context) {
	counter, err := meter.AsyncInt64().UpDownCounter(
		"some.prefix.up_down_counter",
		instrument.WithUnit("1"),
		instrument.WithDescription("TODO"),
	)
	if err != nil {
		panic(err)
	}

	if err := meter.RegisterCallback(
		[]instrument.Asynchronous{
			counter,
		},
		func(ctx context.Context) {
			num := runtime.NumGoroutine()
			counter.Observe(ctx, int64(num))
		},
	); err != nil {
		panic(err)
	}
}

// gaugeObserver demonstrates how to measure non-additive numbers that can go up and down,
// for example, cache hit rate or memory utilization.
func gaugeObserver(ctx context.Context) {
	gauge, _ := meter.AsyncFloat64().Gauge(
		"some.prefix.gauge_observer",
		instrument.WithUnit("1"),
		instrument.WithDescription("TODO"),
	)

	if err := meter.RegisterCallback(
		[]instrument.Asynchronous{
			gauge,
		},
		func(ctx context.Context) {
			gauge.Observe(ctx, rand.Float64())
		},
	); err != nil {
		panic(err)
	}
}

// counterWithLabels demonstrates how to add different labels ("hits" and "misses")
// to measurements. Using this simple trick, you can get number of hits, misses,
// sum = hits + misses, and hit_rate = hits / (hits + misses).
func counterWithLabels(ctx context.Context) {
	counter, _ := meter.SyncInt64().Counter(
		"some.prefix.cache",
		instrument.WithDescription("Cache hits and misses"),
	)
	for {
		if rand.Float64() < 0.3 {
			// increment hits
			counter.Add(ctx, 1, attribute.String("type", "hits"))
		} else {
			// increments misses
			counter.Add(ctx, 1, attribute.String("type", "misses"))
		}

		time.Sleep(time.Millisecond)
	}
}

// counterObserverAdvanced demonstrates how to measure monotonic (non-decreasing) numbers,
// for example, number of requests or connections.
func counterObserverAdvanced(ctx context.Context) {
	// stats is our data source updated by some library.
	var stats struct {
		Hits   int64 // atomic
		Misses int64 // atomic
	}

	hitsCounter, _ := meter.AsyncInt64().Counter("some.prefix.cache_hits")
	missesCounter, _ := meter.AsyncInt64().Counter("some.prefix.cache_misses")

	if err := meter.RegisterCallback(
		[]instrument.Asynchronous{
			hitsCounter,
			missesCounter,
		},
		// SDK periodically calls this function to collect data.
		func(ctx context.Context) {
			hitsCounter.Observe(ctx, atomic.LoadInt64(&stats.Hits))
			missesCounter.Observe(ctx, atomic.LoadInt64(&stats.Misses))
		},
	); err != nil {
		panic(err)
	}

	for {
		if rand.Float64() < 0.3 {
			atomic.AddInt64(&stats.Misses, 1)
		} else {
			atomic.AddInt64(&stats.Hits, 1)
		}

		time.Sleep(time.Millisecond)
	}
}
