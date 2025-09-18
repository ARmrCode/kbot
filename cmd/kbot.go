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

	"github.com/hirosassa/zerodriver"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

var (
	TeleToken   = os.Getenv("TELE_TOKEN")
	MetricsHost = os.Getenv("METRICS_HOST") // например: localhost:4317
)

// Инициализация OpenTelemetry OTLP метрик
func initOTLP(ctx context.Context) {
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(MetricsHost),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("failed to create OTLP exporter: %v", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second)),
		),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(fmt.Sprintf("kbot_%s", appVersion)),
		)),
	)

	otel.SetMeterProvider(mp)
}

// Инициализация Prometheus метрик
func initPrometheus() *prometheus.Exporter {
	config := prometheus.Config{}
	exporter, err := prometheus.New(config, sdkmetric.NewMeterProvider())
	if err != nil {
		log.Fatalf("failed to create Prometheus exporter: %v", err)
	}

	// Поднимаем HTTP-сервер для /metrics
	mux := http.NewServeMux()
	mux.Handle("/metrics", exporter)

	go func() {
		addr := ":9464" // стандартный порт для OTEL Prometheus exporter
		log.Printf("Prometheus metrics listening on %s/metrics", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("failed to start Prometheus metrics server: %v", err)
		}
	}()

	return exporter
}

// Увеличение счётчика метрик
func recordMetric(ctx context.Context, payload string) {
	meter := otel.GetMeterProvider().Meter("kbot_light_signal_counter")
	counter, _ := meter.Int64Counter(fmt.Sprintf("kbot_light_signal_%s", payload))
	counter.Add(ctx, 1)
}

var kbotCmd = &cobra.Command{
	Use:     "kbot",
	Aliases: []string{"start"},
	Short:   "Telegram bot with metrics",
	Run: func(cmd *cobra.Command, args []string) {
		logger := zerodriver.NewProductionLogger()

		kbot, err := telebot.NewBot(telebot.Settings{
			Token:  TeleToken,
			Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
		})
		if err != nil {
			logger.Fatal().Str("Error", err.Error()).Msg("Please check TELE_TOKEN")
		} else {
			logger.Info().Str("Version", appVersion).Msg("kbot started")
		}

		initOTLP(context.Background())
		initPrometheus()

		trafficSignal := map[string]map[string]int8{
			"red":   {"pin": 12, "on": 0},
			"amber": {"pin": 27, "on": 0},
			"green": {"pin": 22, "on": 0},
		}

		kbot.Handle(telebot.OnText, func(m telebot.Context) error {
			payload := m.Message().Payload
			logger.Info().Str("Payload", payload).Msg("Received")

			recordMetric(context.Background(), payload)

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
	rootCmd.AddCommand(kbotCmd)
}
