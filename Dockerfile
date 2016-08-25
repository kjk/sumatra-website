FROM alpine:3.4

WORKDIR /app

COPY sumatra_website_linux /app/
COPY www /app/www/

EXPOSE 80

CMD ["./sumatra_website_linux", "-addr=:80"]
