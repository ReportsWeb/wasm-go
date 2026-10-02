FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN go build -o /quick-start .
FROM alpine:3.22
WORKDIR /app
COPY --from=build /quick-start /app/quick-start
COPY index.html /app/index.html
COPY definition /app/definition
ENTRYPOINT ["/app/quick-start"]
