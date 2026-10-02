FROM golang:1.25-bookworm AS build
WORKDIR /work
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/reports-go .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 reports
WORKDIR /opt/reports
COPY --from=build /out/reports-go ./reports-go
COPY public ./public
COPY resources ./resources
USER reports
ENV HOST=0.0.0.0 PORT=8080 REPORTS_RESOURCE_ROOT=/opt/reports/resources REPORTS_TEMPLATE=/opt/reports/public/index.html REPORTS_WEB_ROOT=/opt/reports/web
EXPOSE 8080
ENTRYPOINT ["/opt/reports/reports-go"]
