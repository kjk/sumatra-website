FROM alpine:3.4

WORKDIR /app

COPY sumatra_website_linux /app/
COPY scripts/entrypoint.sh /app/entrypoint.sh
COPY www /app/www/

EXPOSE 80

CMD ["./entrypoint.sh"]
