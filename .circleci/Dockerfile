FROM alpine:3.6
LABEL maintainer="jimmidyson@gmail.com"
EXPOSE 8080
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY pure-bot .
USER 10000
CMD ["/pure-bot"]
