/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2017 PayinTech
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */
package caddy_datadog

import (
	"github.com/DataDog/datadog-go/statsd"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"regexp"
	"time"
)

const (
	SETTINGS_DEFAULT_STATSD = "127.0.0.1:8125"
	TICKER_INTERVAL         = 10.0
	STATSD_RATE             = 1.0
)

type DatadogModule struct {
	Next          httpserver.Handler
	daemonAddress string
	tags          []string
}

type DatadogMetrics struct {
	response1xxCount float64
	response2xxCount float64
	response3xxCount float64
	response4xxCount float64
	response5xxCount float64
	responseSize     float64
	responseTime     int64
}

var glDatadogMetrics *DatadogMetrics

func initializeDatadogHQ(controller *caddy.Controller) error {
	datadog := &DatadogModule{
		daemonAddress: SETTINGS_DEFAULT_STATSD,
		tags:          []string{},
	}
	if glDatadogMetrics == nil {
		ipAddressRegex := regexp.MustCompile(`^[0-9]{1,3}.[0-9]{1,3}.[0-9]{1,3}.[0-9]{1,3}:[0-9]{1,5}$`)
		tagRegex := regexp.MustCompile(`^[a-zA-Z0-9:]{1,25}$`)

		for controller.Next() {
			for controller.NextBlock() {
				if glDatadogMetrics == nil {
					glDatadogMetrics = &DatadogMetrics{}
				}
				switch controller.Val() {
				case "statsd":
					datadog.daemonAddress = controller.RemainingArgs()[0]
					if !ipAddressRegex.MatchString(datadog.daemonAddress) {
						return controller.Err("datadog: not a valid address. Must be <ip>:<port>")
					}
				case "tags":
					datadog.tags = controller.RemainingArgs()
					for idx, tag := range datadog.tags {
						if !tagRegex.MatchString(tag) {
							return controller.Errf("datadog: tag #%d is not valid", idx+1)
						}
					}
				}
			}
		}

		daemonClient, err := statsd.New(datadog.daemonAddress)
		if err != nil {
			return err
		}

		daemonClient.Namespace = "caddy."
		daemonClient.Tags = datadog.tags

		ticker := time.NewTicker(time.Second * TICKER_INTERVAL)
		quit := make(chan struct{})
		go func() {
			for {
				select {
				case <-ticker.C:
					totalResponsesPeriod := glDatadogMetrics.response1xxCount +
						glDatadogMetrics.response2xxCount +
						glDatadogMetrics.response3xxCount +
						glDatadogMetrics.response4xxCount +
						glDatadogMetrics.response5xxCount
					daemonClient.Gauge(
						"requests.per_s",
						totalResponsesPeriod/TICKER_INTERVAL,
						nil,
						STATSD_RATE,
					)
					daemonClient.Gauge(
						"responses.1xx",
						glDatadogMetrics.response1xxCount,
						nil,
						STATSD_RATE,
					)
					daemonClient.Gauge(
						"responses.2xx",
						glDatadogMetrics.response2xxCount,
						nil,
						STATSD_RATE,
					)
					daemonClient.Gauge(
						"responses.3xx",
						glDatadogMetrics.response3xxCount,
						nil,
						STATSD_RATE,
					)
					daemonClient.Gauge(
						"responses.4xx",
						glDatadogMetrics.response4xxCount,
						nil,
						STATSD_RATE,
					)
					daemonClient.Gauge(
						"responses.5xx",
						glDatadogMetrics.response5xxCount,
						nil,
						STATSD_RATE,
					)
					daemonClient.Gauge(
						"responses.size_bytes",
						glDatadogMetrics.responseSize,
						nil,
						STATSD_RATE,
					)
					if totalResponsesPeriod == 0 { // Avoid div. per zero
						totalResponsesPeriod = 1
					}
					daemonClient.Gauge(
						"responses.duration",
						float64(glDatadogMetrics.responseTime)/totalResponsesPeriod,
						nil,
						STATSD_RATE,
					)
					glDatadogMetrics.response1xxCount = 0
					glDatadogMetrics.response2xxCount = 0
					glDatadogMetrics.response3xxCount = 0
					glDatadogMetrics.response4xxCount = 0
					glDatadogMetrics.response5xxCount = 0
					glDatadogMetrics.responseSize = 0
					glDatadogMetrics.responseTime = 0
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
