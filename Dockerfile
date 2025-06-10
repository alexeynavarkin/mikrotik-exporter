FROM golang:1.24.3-alpine3.18 AS builder

WORKDIR /build
COPY . .
RUN go build -o mikrotik-exporter ./cmd/main.go

FROM alpine:3.18
WORKDIR /mikrotik-exporter
COPY --from=builder /build/mikrotik-exporter .
CMD [ "./mikrotik-exporter" ]