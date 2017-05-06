package caddy_datadog

import (
	"github.com/DataDog/datadog-go/statsd"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"regexp"
	"strconv"
	"time"
)

func init() {
	caddy.RegisterPlugin("datadog", caddy.Plugin{
		ServerType: "http",
		Action:     initializeDatadogHQ,
	})
}

type DatadogModule struct {
	Next          httpserver.Handler
	daemonAddress string
	tags          []string
	rate          float64
}

type DatadogMetrics struct {
	response1xxCount float64
	response2xxCount float64
	response3xxCount float64
	response4xxCount float64
	response5xxCount float64
	responseSize     float64
}

var glDatadogMetrics *DatadogMetrics

func initializeDatadogHQ(controller *caddy.Controller) error {
	datadog := &DatadogModule{
		daemonAddress: "127.0.0.1:8125",
		tags:          []string{},
		rate:          1.0,
	}
	if glDatadogMetrics == nil {
		ipAddressRegex := regexp.MustCompile(`^[0-9]{1,3}.[0-9]{1,3}.[0-9]{1,3}.[0-9]{1,3}:[0-9]{1,5}$`)
		tagRegex := regexp.MustCompile(`^[a-zA-Z0-9]{1,25}$`)

		glDatadogMetrics = &DatadogMetrics{}
		for controller.Next() {
			for controller.NextBlock() {
				switch controller.Val() {
				case "statsd":
					datadog.daemonAddress = controller.RemainingArgs()[0]
					if !ipAddressRegex.MatchString(datadog.daemonAddress) {
						return controller.Err("datadog: not a valid address")
					}
				case "tags":
					datadog.tags = controller.RemainingArgs()
					for idx, tag := range datadog.tags {
						if !tagRegex.MatchString(tag) {
							return controller.Errf("datadog: tag #%d is not valid", idx)
						}
					}
				case "rate":
					var err = error(nil)
					datadog.rate, err = strconv.ParseFloat(controller.RemainingArgs()[0], 64)
					if err != nil {
						return controller.Err("datadog: not a valid float")
					}
				}
			}
		}

		daemonClient, err := statsd.New(datadog.daemonAddress)
		if err != nil {
			return err
		}
		daemonClient.SimpleEvent(
			"Caddy - The HTTP/2 Web Server with Automatic HTTPS",
			"Caddy server is now running...",
		)
		daemonClient.Namespace = "caddy."
		daemonClient.Tags = datadog.tags

		ticker := time.NewTicker(time.Second * 10)
		quit := make(chan struct{})
		go func() {
			for {
				select {
				case <-ticker.C:
					//fmt.Println(
					//	glDatadogMetrics.response1xxCount,
					//	glDatadogMetrics.response2xxCount,
					//	glDatadogMetrics.response3xxCount,
					//	glDatadogMetrics.response4xxCount,
					//	glDatadogMetrics.response5xxCount,
					//)
					totalResponses := glDatadogMetrics.response1xxCount +
						glDatadogMetrics.response2xxCount +
						glDatadogMetrics.response3xxCount +
						glDatadogMetrics.response4xxCount +
						glDatadogMetrics.response5xxCount
					daemonClient.Gauge(
						"requests.per_s",
						totalResponses,
						nil,
						datadog.rate,
					)
					daemonClient.Gauge(
						"requests.total",
						totalResponses,
						nil,
						datadog.rate,
					)
					daemonClient.Gauge(
						"responses.1xx",
						glDatadogMetrics.response1xxCount,
						nil,
						datadog.rate,
					)
					daemonClient.Gauge(
						"responses.2xx",
						glDatadogMetrics.response2xxCount,
						nil,
						datadog.rate,
					)
					daemonClient.Gauge(
						"responses.3xx",
						glDatadogMetrics.response3xxCount,
						nil,
						datadog.rate,
					)
					daemonClient.Gauge(
						"responses.4xx",
						glDatadogMetrics.response4xxCount,
						nil,
						datadog.rate,
					)
					daemonClient.Gauge(
						"responses.5xx",
						glDatadogMetrics.response5xxCount,
						nil,
						datadog.rate,
					)
					daemonClient.Gauge(
						"responses.size_bytes",
						glDatadogMetrics.responseSize,
						nil,
						datadog.rate,
					)
					glDatadogMetrics.response1xxCount = 0
					glDatadogMetrics.response2xxCount = 0
					glDatadogMetrics.response3xxCount = 0
					glDatadogMetrics.response4xxCount = 0
					glDatadogMetrics.response5xxCount = 0
					glDatadogMetrics.responseSize = 0
				case <-quit:
					ticker.Stop()
					return
				}
			}
		}()
	}

	httpserver.
		GetConfig(controller).
		AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
			datadog.Next = next
			return datadog
		})

	return nil
}
