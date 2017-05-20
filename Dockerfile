FROM alpine:3.4

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY sumatra_website_linux /app/sumatra_website
COPY www /app/www/

EXPOSE 80 443

CMD ["/app/sumatra_website", "-production"]
