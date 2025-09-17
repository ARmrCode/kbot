package cmd

import (
    "context"
    "fmt"
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
    MetricsHost = os.Getenv("METRICS_HOST") // OTLP gRPC endpoint
    PromPort    = os.Getenv("PROM_PORT")    // Prometheus HTTP metrics port, например ":2222"
)

// Инициализация метрик OTLP + Prometheus
func initMetrics(ctx context.Context) {
    // OTLP gRPC экспортёр
    otlpExporter, _ := otlpmetricgrpc.New(
        ctx,
        otlpmetricgrpc.WithEndpoint(MetricsHost),
        otlpmetricgrpc.WithInsecure(),
    )

    // Prometheus экспортёр
    promExporter, _ := prometheus.New(
        prometheus.WithNamespace("kbot"),
        prometheus.WithPort(PromPort),
    )

    res := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String(fmt.Sprintf("kbot_%s", appVersion)),
    )

    meterProvider := sdkmetric.NewMeterProvider(
        sdkmetric.WithResource(res),
        sdkmetric.WithReader(
            sdkmetric.NewPeriodicReader(otlpExporter, sdkmetric.WithInterval(10*time.Second)),
        ),
    )

    // Установка глобального MeterProvider
    otel.SetMeterProvider(meterProvider)

    // Также Prometheus напрямую
    go func() {
        _ = promExporter.Serve()
    }()
}

// Функция увеличения счетчиков
func recordMetric(ctx context.Context, payload string) {
    meter := otel.GetMeterProvider().Meter("kbot_metrics")
    counter, _ := meter.Int64Counter(fmt.Sprintf("kbot_signal_%s", payload))
    counter.Add(ctx, 1)
}

// Kbot command
var kbotCmd = &cobra.Command{
    Use:   "kbot",
    Short: "Telegram bot with metrics",
    Run: func(cmd *cobra.Command, args []string) {
        ctx := context.Background()
        kbot, _ := telebot.NewBot(telebot.Settings{
            Token:  TeleToken,
            Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
        })

        trafficSignal := map[string]map[string]int8{
            "red":   {"pin": 12, "on": 0},
            "amber": {"pin": 27, "on": 0},
            "green": {"pin": 22, "on": 0},
        }

        kbot.Handle(telebot.OnText, func(m telebot.Context) error {
            payload := m.Message().Payload
            recordMetric(ctx, payload)

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
