FROM golang:1.25-alpine AS build

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/main ./cmd/app/

FROM scratch AS runner

COPY --from=build /app/main /app/main
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/app/main"]