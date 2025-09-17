/*
Copyright © 2025 NAME HERE <EMAIL>
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
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

var (
	TeleToken   = os.Getenv("TELE_TOKEN")
	MetricsHost = os.Getenv("METRICS_HOST") // для OTLP
)

// Инициализация метрик OpenTelemetry
func initMetrics(ctx context.Context) (*prometheus.Exporter, error) {
	// OTLP gRPC exporter
	if MetricsHost != "" {
		exporter, err := otlpmetricgrpc.New(
			ctx,
			otlpmetricgrpc.WithEndpoint(MetricsHost),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
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
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	// Запуск HTTP сервера для Prometheus метрик
	go func() {
		http.Handle("/metrics", promExporter)
		log.Println("Prometheus metrics available at :8889/metrics")
		log.Fatal(http.ListenAndServe(":8889", nil))
	}()

	return promExporter, nil
}

// Отправка метрик для payload
func recordMetric(ctx context.Context, payload string) {
	meter := otel.GetMeterProvider().Meter("kbot_light_signal_counter")
	counter, _ := meter.Int64Counter(fmt.Sprintf("kbot_light_signal_%s", payload))
	counter.Add(ctx, 1)
}

// Команда kbot
var kbotCmd = &cobra.Command{
	Use:     "kbot",
	Aliases: []string{"start"},
	Short:   "Start Kbot Telegram bot with metrics",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		_, err := initMetrics(ctx)
		if err != nil {
			log.Fatalf("Failed to init metrics: %v", err)
		}

		kbot, err := telebot.NewBot(telebot.Settings{
			Token:  TeleToken,
			Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
		})
		if err != nil {
			log.Fatalf("Check TELE_TOKEN: %v", err)
		}

		log.Printf("Kbot started, version %s", appVersion)

		trafficSignal := map[string]map[string]int8{
			"red":   {"pin": 12, "on": 0},
			"amber": {"pin": 27, "on": 0},
			"green": {"pin": 22, "on": 0},
		}

		kbot.Handle(telebot.OnText, func(m telebot.Context) error {
			payload := m.Message().Payload
			if payload == "" {
				payload = m.Text()
			}

			recordMetric(ctx, payload)

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
	rootCmd.AddCommand(kbotCmd)
}
