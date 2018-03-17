FROM alpine:3.6
LABEL maintainer="dev@syndesis.io"
EXPOSE 8080
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY pure-bot .
ENTRYPOINT ["/pure-bot"]
USER 10000
