# Datadog plugin for Caddy HTTP/2 web server

Datadog plugin allow Caddy HTTP/2 web server to send some metrics to Datadog via statsd.
*****


## Configuration
In your Caddy configuration file, you have to use the directive `datadog`
to enable Datadog metric harvester on each configuration blocks where it
was needed.

In the following example, all requests on _example-d.com_ won't be harvested.

    example-a.com {
      datadog "area"              # area is optional
    }

    example-b.com {
      datadog "area" {            # area is optional
        statsd    127.0.0.1:8125  # Optional - can be any valid hostname with port
        tags      tag1 tag2 tagN  # Optional
        namespace caddy.          # Optional
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

| Metric                    | Type  | Unit                |
| ------------------------- | ----- | ------------------- |
| caddy.requests.per_s      | Gauge | requests per second |
| caddy.responses.1xx       | Gauge | requests            |
| caddy.responses.2xx       | Gauge | requests            |
| caddy.responses.3xx       | Gauge | requests            |
| caddy.responses.4xx       | Gauge | requests            |
| caddy.responses.5xx       | Gauge | requests            |
| caddy.responses.size_byte | Gauge | bytes               |
| caddy.responses.duration  | Gauge | nanoseconds         |



## License
This project is released under terms of the [MIT license](https://raw.githubusercontent.com/payintech/caddy-datadog/master/LICENSE).
