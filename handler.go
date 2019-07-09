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
	"github.com/caddyserver/caddy/caddyhttp/httpserver"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"net/http"
	"sync/atomic"
	"time"
)

func (datadog DatadogModule) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) (int, error) {
	timeStart := time.Now()
	responseRecorder := httpserver.NewResponseRecorder(responseWriter)
	spanContext, _ := tracer.Extract(tracer.HTTPHeadersCarrier(request.Header))
	spanOptions := []ddtrace.StartSpanOption{
		tracer.ResourceName(request.Method), // TODO: change this to a more useful resource name
		tracer.SpanType(ext.SpanTypeWeb),
		tracer.ChildOf(spanContext),
		tracer.Tag(ext.HTTPMethod, request.Method),
		tracer.Tag(ext.HTTPURL, request.URL.Path),
	}
	span := tracer.StartSpan("caddy.request", spanOptions...)
	tracer.Inject(span.Context(), tracer.HTTPHeadersCarrier(request.Header))
	status, err := datadog.Next.ServeHTTP(responseRecorder, request)
	atomic.AddUint64(&datadog.Metrics.responseTime, uint64(time.Since(timeStart).Nanoseconds()))

	var realStatus = status
	if realStatus == 0 {
		realStatus = responseRecorder.Status()
	}

	span.SetTag(ext.HTTPCode, realStatus)
	span.SetTag(ext.Error, err)
	span.Finish()

	atomic.AddUint64(&datadog.Metrics.responseSize, uint64(responseRecorder.Size()))
	switch realStatus / 100 {
	case 1:
		atomic.AddUint64(&datadog.Metrics.response1xxCount, 1)
		break
	case 2:
		atomic.AddUint64(&datadog.Metrics.response2xxCount, 1)
		break
	case 3:
		atomic.AddUint64(&datadog.Metrics.response3xxCount, 1)
		break
	case 4:
		atomic.AddUint64(&datadog.Metrics.response4xxCount, 1)
		break
	default:
		atomic.AddUint64(&datadog.Metrics.response5xxCount, 1)
		break
	}

	return status, err
}
