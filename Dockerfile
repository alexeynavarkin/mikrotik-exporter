FROM golang:1.24.3-alpine3.20 AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o mikrotik-exporter ./cmd

FROM alpine:3.20
WORKDIR /mikrotik-exporter
COPY --from=builder /build/mikrotik-exporter .
USER nobody
EXPOSE 9100
CMD [ "./mikrotik-exporter" ]
