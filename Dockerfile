########## Builder stage ##########
FROM golang:1.25-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build -o /out/markdown-to-pdf ./cmd/markdown-to-pdf && \
    CGO_ENABLED=0 go build -o /out/files-dashboard ./cmd/files-dashboard

########## Runtime stage ##########
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    zip ca-certificates chromium chromium-driver && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
ENV CHROME_BIN=/usr/bin/chromium
ENV CHROMEDP_DISABLE_GPU=true
WORKDIR /github/workspace
COPY --from=builder /out/markdown-to-pdf /usr/local/bin/markdown-to-pdf
COPY --from=builder /out/files-dashboard /usr/local/bin/files-dashboard
COPY entrypoint.sh /usr/local/bin/run-action.sh
RUN chmod +x /usr/local/bin/run-action.sh
ENTRYPOINT ["/usr/local/bin/run-action.sh"]