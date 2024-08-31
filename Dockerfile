FROM alpine:3.20.2

WORKDIR /app

COPY simple_cdn /user/bin/
ENTRYPOINT ["/usr/bin/simple_cdn"]
