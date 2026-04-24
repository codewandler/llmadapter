# syntax=docker/dockerfile:1.7

FROM golang:1.26.1-alpine AS build

RUN apk add --no-cache ca-certificates git
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/llmadapter ./cmd/llmadapter

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 llmadapter

COPY --from=build /out/llmadapter /usr/local/bin/llmadapter

USER llmadapter
ENV LLMADAPTER_ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/llmadapter"]
CMD ["serve", "--addr", ":8080"]
