FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o restaurant-system ./cmd/restaurant-system

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/restaurant-system /app/restaurant-system
COPY config.yaml /app/config.yaml
CMD ["/app/restaurant-system","--mode=order-service","--port=3000"]
