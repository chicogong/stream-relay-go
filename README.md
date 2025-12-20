# stream-relay-go
A Go-based streaming relay/gateway for LLM and TTS APIs. It transparently proxies SSE and raw byte streams while adding production-grade observability (TTFT/TTFA, TPS, bytes, stop reasons, error taxonomy, token/usage accounting) and policy controls (auth, routing, rate limiting, circuit breaking), exporting metrics to Prometheus and traces via OpenTelemetry.
