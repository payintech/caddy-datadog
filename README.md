# Datadog plugin for Caddy HTTP/2 web server

Datadog plugin allow Caddy HTTP/2 web server to send some metrics to Datadog via statsd.
*****


## Configuration
In your Caddy configuration file, you have to use the directive `datadog`
to enable Datadog metric harvester on each configuration blocks where it
was needed.

In the following example, all requests on _example-d.com_ won't be harvested.

    example-a.com {
      datadog
    }

    example-b.com {
      datadog {
        statsd 127.0.0.1:8125
        tags tag1 tag2 tag3
        rate 1
      }
    }

    example-c.com {
      datadog
    }

    example-d.com {
    }

**Note:** As you can see on the previous example, the directive `datadog`
can be configured only once.



## Metrics
The plugin send following metrics to Datadog:

  - caddy.requests.per_s
  - caddy.responses.1xx
  - caddy.responses.2xx
  - caddy.responses.3xx
  - caddy.responses.4xx
  - caddy.responses.5xx
  - caddy.responses.size_bytes
