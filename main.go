package caddy_datadog

import "github.com/mholt/caddy"

func init() {
	caddy.RegisterPlugin("datadog", caddy.Plugin{
		ServerType: "http",
		Action:     initializeDatadogHQ,
	})
}
