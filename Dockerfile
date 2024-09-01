FROM alpine:3.20.2

WORKDIR /app

COPY simple_cdn /usr/bin/
ENTRYPOINT ["/usr/bin/simple_cdn"]
