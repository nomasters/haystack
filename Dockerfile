FROM golang:1.24.6-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 for fully static binary compatible with scratch
# -ldflags="-w -s" strips debug info for smaller size
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -o haystack \
    ./cmd/haystack

FROM scratch

COPY --from=builder /build/haystack /haystack

# scratch doesn't have mkdir, so WORKDIR creates the directory
WORKDIR /data

EXPOSE 1337/udp

ENTRYPOINT ["/haystack"]
