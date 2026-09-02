# ----- build stage -----
FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o renobot .

# ----- runtime stage (distroless) -----
FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /build/renobot /renobot

USER nonroot:nonroot

ENTRYPOINT ["/renobot"]
CMD ["--config", "/config.yaml"]
