FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /openflux-panel .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates iproute2
WORKDIR /app
COPY --from=build /openflux-panel /app/openflux-panel
COPY web /app/web
VOLUME /app/data
EXPOSE 8080
ENV PANEL_PORT=8080 PANEL_DATA=/app/data
CMD ["/app/openflux-panel", "-port", "8080", "-data", "/app/data"]
