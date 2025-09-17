/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	telebot "gopkg.in/telebot.v3"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/prometheus/promhttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

var (
	TeleToken   = os.Getenv("TELE_TOKEN")
	MetricsHost = os.Getenv("METRICS_HOST")
)

// Initialize OpenTelemetry
func initMetrics(ctx context.Context) {
	// OTLP Metric exporter
	if MetricsHost != "" {
		exporter, err := otlpmetricgrpc.New(
			ctx,
			otlpmetricgrpc.WithEndpoint(MetricsHost),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			log.Fatalf("Failed to create OTLP exporter: %v", err)
		}

		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(fmt.Sprintf("kbot_%s", appVersion)),
			)),
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second))),
		)

		otel.SetMeterProvider(mp)
	}

	// Prometheus exporter
	promExporter, err := prometheus.New(prometheus.WithoutUnits())
	if err != nil {
		log.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler(promExporter))
		addr := ":8889"
		log.Printf("Prometheus metrics available at %s/metrics", addr)
		log.Fatal(http.ListenAndServe(addr, mux))
	}()
}

// Increment metric for payload
func pmetrics(ctx context.Context, payload string) {
	meter := otel.GetMeterProvider().Meter("kbot_light_signal_counter")
	counter, _ := meter.Int64Counter(fmt.Sprintf("kbot_light_signal_%s", payload))
	counter.Add(ctx, 1)
}

// kbotCmd represents the kbot command
var kbotCmd = &cobra.Command{
	Use:     "kbot",
	Aliases: []string{"start"},
	Short:   "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		kbot, err := telebot.NewBot(telebot.Settings{
			URL:    "",
			Token:  TeleToken,
			Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
		})
		if err != nil {
			log.Fatalf("Please check TELE_TOKEN env variable: %v", err)
		}

		trafficSignal := map[string]map[string]int8{
			"red":   {"pin": 12},
			"amber": {"pin": 27},
			"green": {"pin": 22},
		}

		for k := range trafficSignal {
			trafficSignal[k]["on"] = 0
		}

		kbot.Handle(telebot.OnText, func(m telebot.Context) error {
			payload := m.Message().Payload
			log.Printf("Payload: %s", payload)

			pmetrics(context.Background(), payload)

			switch payload {
			case "hello":
				return m.Send(fmt.Sprintf("Hello I'm Kbot %s!", appVersion))
			case "red", "amber", "green":
				if trafficSignal[payload]["on"] == 0 {
					trafficSignal[payload]["on"] = 1
				} else {
					trafficSignal[payload]["on"] = 0
				}
				return m.Send(fmt.Sprintf("Switch %s light signal to %d", payload, trafficSignal[payload]["on"]))
			default:
				return m.Send("Usage: /s red|amber|green")
			}
		})

		kbot.Start()
	},
}

func init() {
	ctx := context.Background()
	initMetrics(ctx)
	rootCmd.AddCommand(kbotCmd)
}
