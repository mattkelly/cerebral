FROM golang:1.12.1-alpine3.9 as builder

# Add build tools
# bzr is required for InfluxDB dependency
RUN apk update && \
    apk add --no-cache git bzr

WORKDIR /app

# Install dependencies first for cache
COPY go.mod go.sum ./
RUN go mod download

# Add the source code
COPY . .

# Build it:
ARG GIT_DESCRIBE
ARG GIT_COMMIT
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -ldflags \
        "-X github.com/containership/cerebral/pkg/buildinfo.gitDescribe=${GIT_DESCRIBE} \
        -X github.com/containership/cerebral/pkg/buildinfo.gitCommit=${GIT_COMMIT} \
        -X github.com/containership/cerebral/pkg/buildinfo.unixTime=`date '+%s'` \
        -w" \
        -a -tags netgo \
        -o cerebral cmd/cerebral/cerebral.go

# Create Docker image of just the binary
FROM scratch as runner
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/cerebral .

CMD ["./cerebral"]
