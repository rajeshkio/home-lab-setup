# syntax=docker/dockerfile:1
FROM golang:1.24.3 as builder

WORKDIR /app

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o api-service

# Runtime stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /app/api-service /api-service

EXPOSE 5000

ENTRYPOINT ["/api-service"]
