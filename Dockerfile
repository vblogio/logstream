# ---- Compilation ----
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY web ./web
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/logwell .

# ---- Image finale (~15 Mo) ----
FROM alpine:3.20
RUN adduser -D -H -u 10001 logwell \
 && mkdir /data && chown logwell /data
COPY --from=build /out/logwell /usr/local/bin/logwell
USER logwell
VOLUME /data
# 5514 en interne (port non privilégié), publié sur 514 par docker-compose
EXPOSE 5514/udp 5514/tcp 8080/tcp
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/logwell"]
