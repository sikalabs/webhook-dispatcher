FROM golang:1.25 AS build
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build

FROM debian:13-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /usr/local/bin/webhook-to-redis .
ENV CONFIG=/config.yaml
CMD ["/usr/local/bin/webhook-to-redis", "server"]
EXPOSE 8000
