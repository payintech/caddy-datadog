package caddy_datadog

import (
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"net/http"
)

func (datadog DatadogModule) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) (int, error) {
	rw := httpserver.NewResponseRecorder(responseWriter)
	status, err := datadog.Next.ServeHTTP(rw, request)

	switch status / 100 {
	case 1:
		glDatadogMetrics.response1xxCount += 1
	case 2:
		glDatadogMetrics.response2xxCount += 1
	case 3:
		glDatadogMetrics.response3xxCount += 1
	case 4:
		glDatadogMetrics.response4xxCount += 1
	default:
		glDatadogMetrics.response5xxCount += 1
	}
	return status, err
}
