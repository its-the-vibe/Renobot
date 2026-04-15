# ----- build stage -----
FROM golang:1.26.2 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o renobot .

# ----- runtime stage -----
FROM scratch

# TLS certificates for any outbound HTTPS calls made by dependencies.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /build/renobot /renobot

ENTRYPOINT ["/renobot"]
CMD ["--config", "/config.yaml"]
