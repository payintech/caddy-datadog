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
	"container/list"
	"github.com/DataDog/datadog-go/statsd"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

const (
	STATSD_SERVER    = "127.0.0.1:8125"
	STATSD_RATE      = 1.0
	STATSD_NAMESPACE = "caddy."
	TICKER_INTERVAL  = 10.0
)

type DatadogModule struct {
	Next    httpserver.Handler
	Metrics *DatadogMetrics
}

type DatadogMetrics struct {
	// Known Go bug: https://golang.org/pkg/sync/atomic/#pkg-note-BUG
	// must be first field for 64 bit alignment
	// on x86 and arm.
	index            int64
	response1xxCount uint64
	response2xxCount uint64
	response3xxCount uint64
	response4xxCount uint64
	response5xxCount uint64
	responseSize     uint64
	responseTime     uint64
	area             []string
}

// Handle to collected metrics.
var glDatadogMetrics list.List

// Handle to the statsd client.
var glStatsdClient *statsd.Client = nil

// Handle to the plugin ticker
var glTicker *time.Ticker

// Try to retrieve metrics for the given area. If no existing
// metrics was found, a newly created metrics structure will
// be returned.
func getOrCreateMetrics(area []string) *DatadogMetrics {
	if area != nil && len(area) == 0 {
		return getOrCreateMetrics(nil)
	}
	for e := glDatadogMetrics.Front(); e != nil; e = e.Next() {
		if reflect.DeepEqual(e.Value.(*DatadogMetrics).area, area) {
			return e.Value.(*DatadogMetrics)
		}
	}
	newMetrics := &DatadogMetrics{
		area: area,
	}
	glDatadogMetrics.PushBack(newMetrics)
	return newMetrics
}

// Reconfigure statsd client with the last know configuration.
// If statsd is already configured; the current connection will
// be closed and a statsd client will be reconfigured.
func reconfigureStatsdClient(server string, namespace string, tags []string) error {
	var err = error(nil)
	if server != "" {
		if glStatsdClient != nil {
			glStatsdClient.Close()
		}
		glStatsdClient, err = statsd.New(server)
	}
	if tags == nil || len(tags) == 0 {
		glStatsdClient.Tags = nil
	} else {
		glStatsdClient.Tags = tags
	}
	if namespace == "" {
		glStatsdClient.Namespace = STATSD_NAMESPACE
	} else {
		glStatsdClient.Namespace = namespace
	}
	return err
}

// Initialize the Datadog module by parsing the current Caddy
// configuration file.
func initializeDatadogHQ(controller *caddy.Controller) error {
	hostnameRegex := regexp.MustCompile(`^[0-9a-zA-Z\\._-]{1,35}:[0-9]{1,5}$`)
	tagRegex := regexp.MustCompile(`^[a-zA-Z0-9:]{1,25}$`)
	namespaceRegex := regexp.MustCompile(`^[a-zA-Z0-9\\.\\-_]{2,25}$`)

	if glStatsdClient == nil {
		reconfigureStatsdClient(STATSD_SERVER, STATSD_NAMESPACE, nil)
	}

	currentDatadogModule := DatadogModule{}
	for controller.Next() {
		currentDatadogModule.Metrics = getOrCreateMetrics(controller.RemainingArgs())

		var statsdServer, statsdNamespace, statsdTags = "", glStatsdClient.Namespace, glStatsdClient.Tags
		for controller.NextBlock() {
			switch controller.Val() {
			case "statsd":
				var args = controller.RemainingArgs()
				if len(args) > 0 {
					statsdServer = args[0]
				} else {
					statsdServer = STATSD_SERVER
				}
				if !hostnameRegex.MatchString(statsdServer) {
					return controller.Err("datadog: not a valid address. Must be <hostname>:<port>")
				}
			case "tags":
				statsdTags = controller.RemainingArgs()
				for idx, tag := range statsdTags {
					if !tagRegex.MatchString(tag) {
						return controller.Errf("datadog: tag #%d is not valid", idx+1)
					}
				}
			case "namespace":
				var args = controller.RemainingArgs()
				if len(args) > 0 {
					statsdNamespace = args[0]
				} else {
					statsdNamespace = STATSD_NAMESPACE
				}
				if !strings.HasSuffix(statsdNamespace, ".") {
					statsdNamespace += "."
				}
				if !namespaceRegex.MatchString(statsdNamespace) ||
					strings.Contains(statsdNamespace, "..") ||
					strings.HasPrefix(statsdNamespace, ".") {
					return controller.Err("datadog: not a valid namespace")
				}
			}
		}
		reconfigureStatsdClient(statsdServer, statsdNamespace, statsdTags)
	}
	httpserver.
		GetConfig(controller).
		AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
			currentDatadogModule.Next = next
			return currentDatadogModule
		})

	if glTicker == nil {
		glTicker = time.NewTicker(time.Second * TICKER_INTERVAL)
		quit := make(chan struct{})
		go func() {
			for {
				select {
				case <-glTicker.C:
					for e := glDatadogMetrics.Front(); e != nil; e = e.Next() {
						var currentMetrics = e.Value.(*DatadogMetrics)

						totalResponsesPeriod := currentMetrics.response1xxCount +
							currentMetrics.response2xxCount +
							currentMetrics.response3xxCount +
							currentMetrics.response4xxCount +
							currentMetrics.response5xxCount

						glStatsdClient.Gauge(
							"requests.per_s",
							float64(totalResponsesPeriod)/TICKER_INTERVAL,
							currentMetrics.area,
							STATSD_RATE,
						)
						glStatsdClient.Incr(
							"responses.1xx",
							currentMetrics.area,
							float64(currentMetrics.response1xxCount),
						)
						glStatsdClient.Gauge(
							"responses.2xx",
							float64(currentMetrics.response2xxCount),
							currentMetrics.area,
							STATSD_RATE,
						)
						glStatsdClient.Gauge(
							"responses.3xx",
							float64(currentMetrics.response3xxCount),
							currentMetrics.area,
							STATSD_RATE,
						)
						glStatsdClient.Gauge(
							"responses.4xx",
							float64(currentMetrics.response4xxCount),
							currentMetrics.area,
							STATSD_RATE,
						)
						glStatsdClient.Gauge(
							"responses.5xx",
							float64(currentMetrics.response5xxCount),
							currentMetrics.area,
							STATSD_RATE,
						)
						glStatsdClient.Gauge(
							"responses.size_bytes",
							float64(currentMetrics.responseSize),
							currentMetrics.area,
							STATSD_RATE,
						)
						if totalResponsesPeriod == 0 { // Avoid div. per zero
							totalResponsesPeriod = 1
						}
						glStatsdClient.Gauge(
							"responses.duration",
							float64(currentMetrics.responseTime)/float64(totalResponsesPeriod),
							currentMetrics.area,
							STATSD_RATE,
						)

						atomic.AddUint64(&currentMetrics.response1xxCount, -currentMetrics.response1xxCount)
						atomic.AddUint64(&currentMetrics.response2xxCount, -currentMetrics.response2xxCount)
						atomic.AddUint64(&currentMetrics.response3xxCount, -currentMetrics.response3xxCount)
						atomic.AddUint64(&currentMetrics.response4xxCount, -currentMetrics.response4xxCount)
						atomic.AddUint64(&currentMetrics.response5xxCount, -currentMetrics.response5xxCount)
						atomic.AddUint64(&currentMetrics.responseSize, -currentMetrics.responseSize)
						currentMetrics.responseTime = 0
					}
				case <-quit:
					glTicker.Stop()
					return
				}
			}
		}()
	}

	return nil
}
