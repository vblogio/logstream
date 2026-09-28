# ---- Build ----
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY web ./web
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/logstream .

# ---- Final image (~15 MB) ----
FROM alpine:3.20
RUN adduser -D -H -u 10001 logstream \
 && mkdir /data && chown logstream /data
COPY --from=build /out/logstream /usr/local/bin/logstream
USER logstream
VOLUME /data
# 5514 inside the container (unprivileged port), published as 514 by docker-compose
EXPOSE 5514/udp 5514/tcp 8080/tcp
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/logstream"]
