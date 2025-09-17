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
	// TELE_TOKEN для Telegram бота
	TeleToken = os.Getenv("TELE_TOKEN")
	// METRICS_HOST для OTLP метрик
	MetricsHost = os.Getenv("METRICS_HOST")
)

// Инициализация OpenTelemetry
func initMetrics(ctx context.Context) {
	// OTLP метрики
	if MetricsHost != "" {
		exporter, err := otlpmetricgrpc.New(ctx,
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
			sdkmetric.WithReader(
				sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second)),
			),
		)
		otel.SetMeterProvider(mp)
	}

	// Prometheus метрики
	promExporter, err := prometheus.New(prometheus.WithoutUnits())
	if err != nil {
		log.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	// HTTP сервер для Prometheus
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promExporter)
		addr := ":8889"
		log.Printf("Prometheus metrics available at %s/metrics", addr)
		log.Fatal(http.ListenAndServe(addr, mux))
	}()
}

// Функция для подсчета события по payload
func pmetrics(ctx context.Context, payload string) {
	meter := otel.GetMeterProvider().Meter("kbot_light_signal_counter")
	counter, _ := meter.Int64Counter(fmt.Sprintf("kbot_light_signal_%s", payload))
	counter.Add(ctx, 1)
}

// kbotCmd — основная команда Cobra
var kbotCmd = &cobra.Command{
	Use:     "kbot",
	Aliases: []string{"start"},
	Short:   "Telegram bot с метриками OpenTelemetry",
	Run: func(cmd *cobra.Command, args []string) {
		kbot, err := telebot.NewBot(telebot.Settings{
			Token:  TeleToken,
			Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
		})
		if err != nil {
			log.Fatalf("Please check TELE_TOKEN: %v", err)
		}

		log.Printf("kbot %s started", appVersion)

		trafficSignal := map[string]map[string]int8{
			"red":   {"pin": 12, "on": 0},
			"amber": {"pin": 27, "on": 0},
			"green": {"pin": 22, "on": 0},
		}

		kbot.Handle(telebot.OnText, func(m telebot.Context) error {
			payload := m.Message().Payload
			log.Printf("Received payload: %s", payload)

			pmetrics(context.Background(), payload)

			switch payload {
			case "hello":
				return m.Send(fmt.Sprintf("Hello I'm Kbot %s!", appVersion))
			case "red", "amber", "green":
				trafficSignal[payload]["on"] ^= 1
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
