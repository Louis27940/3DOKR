FROM golang:1.27.4-alpine3.23 AS builder

RUN apk --no-cache add ca-certificates && \
    addgroup -S -g 10001 appgroup && \
    adduser -S -u 10001 -G appgroup appuser && \
    mkdir -p /tmp && chown -R 10001:10001 /tmp

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -extldflags '-static'" -o /worker .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder --chown=10001:10001 /tmp /tmp
COPY --from=builder --chown=10001:10001 /worker /worker

USER 10001:10001

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/worker", "-healthcheck"]

ENTRYPOINT ["/worker"]
