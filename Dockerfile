ARG flb_upstream_version=latest

FROM golang:1.24 AS builder

WORKDIR /src/fluent-bit-for-ecs

COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./cmd/ ./cmd/
COPY ./main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -v -trimpath -a -o /fluent-bit-for-ecs ./main.go

FROM fluent/fluent-bit:${flb_upstream_version}

LABEL author="Alexey Zapparov <alexey@zapparov.com>" \
  org.opencontainers.image.authors="Alexey Zapparov <alexey@zapparov.com>" \
  org.opencontainers.image.description="Fluent-Bit for ECS" \
  org.opencontainers.image.licenses="Apache-2.0 AND MIT" \
  org.opencontainers.image.source="https://github.com/ixti/fluent-bit-for-ecs"

ENV FLB_LOG_LEVEL=info

COPY --from=builder /fluent-bit-for-ecs /fluent-bit-for-ecs
COPY ./LICENSE /fluent-bit-for-ecs.LICENSE
COPY ./conf/ /fluent-bit/etc/

# This is for reference only, you will need to specify HealthCheck on container
# definition of a task, e.g.
# [source,yaml]
# ----
# HealthCheck:
#   Command:     ["CMD", "/fluent-bit-for-ecs", "health"]
#   Interval:    10
#   StartPeriod: 30
#   Retries:     3
#   Timeout:     3
# ----
HEALTHCHECK --interval=10s --timeout=3s CMD /fluent-bit-for-ecs health

ENTRYPOINT ["/fluent-bit-for-ecs", "exec"]

CMD ["/fluent-bit/bin/fluent-bit", "-c", "/fluent-bit/etc/fluent-bit.yml"]
