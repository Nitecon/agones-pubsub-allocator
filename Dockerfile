FROM golang:alpine AS builder

ENV GO111MODULE=auto \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64


RUN apk --update add ca-certificates git

WORKDIR /workspace
# Copy the Go Modules manifests
COPY . /workspace
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Build
RUN go build -ldflags "-s -w -X main.version=$(git branch | sed -n -e 's/^\* \(.*\)/\1/p')" -o /app cmd/main.go

FROM alpine:3.14

WORKDIR /

# Copy in your certificates and passwd file first
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

RUN addgroup -S appusr && adduser -S appusr -G appusr
COPY --from=builder /app .
USER appusr

ENTRYPOINT ["/app"]
EXPOSE 8080