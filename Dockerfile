############################
# STEP 1 build executable binary
############################
FROM golang:alpine AS builder
# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates
# Create appuser.
ENV USER=appuser
ENV UID=10001
# See https://stackoverflow.com/a/55757473/12429735RUN
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"
WORKDIR $GOPATH/src/github.com/ceres-ventures/fonzie/
COPY . .
# Fetch dependencies.
# Using go get.
RUN go mod download
# Using go mod.
# RUN go mod download
# RUN go mod verify
# Build the binary.
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/fonzie .

############################
# STEP 2 build a small image
############################
FROM scratch
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Import the user and group files from the builder.
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
# Copy our static executable.
COPY --from=builder /go/bin/fonzie /go/bin/fonzie
# Use an unprivileged user.
USER appuser:appuser
ENV BIND_IP=0.0.0.0
ENV BIND_PORT=9292
# Run the hello binary.
ENTRYPOINT ["/go/bin/fonzie"]