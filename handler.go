package caddy_datadog

import (
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"net/http"
)

func (datadog DatadogModule) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) (int, error) {
	responseRecorder := httpserver.NewResponseRecorder(responseWriter)
	status, err := datadog.Next.ServeHTTP(responseRecorder, request)

	var realStatus = status
	if realStatus == 0 {
		realStatus = responseRecorder.Status()
	}

	switch realStatus / 100 {
	case 1:
		glDatadogMetrics.response1xxCount += 1
		break
	case 2:
		glDatadogMetrics.response2xxCount += 1
		break
	case 3:
		glDatadogMetrics.response3xxCount += 1
		break
	case 4:
		glDatadogMetrics.response4xxCount += 1
		break
	default:
		glDatadogMetrics.response5xxCount += 1
		break
	}
	return status, err
}
