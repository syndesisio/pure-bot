FROM golang:1.9.2 as build-stage
ARG VERSION
ARG BUILD_DATE
WORKDIR /go/src/github.com/syndesisio/pure-bot
COPY . .
RUN command -v dep >/dev/null 2>&1 || go get github.com/golang/dep/cmd/dep
RUN dep ensure -v
RUN CGO_ENABLED=0 GOOS=linux go build \
  -a -installsuffix "static" \
  -ldflags "-X $(go list .)/pkg/version.AppVersion=$VERSION -X '$(go list .)/pkg/version.BuildDate=$BUILD_DATE'" \
  -o pure-bot \
  .

FROM alpine:3.6
LABEL maintainer="jimmidyson@gmail.com"
EXPOSE 8080
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY --from=build-stage /go/src/github.com/syndesisio/pure-bot/pure-bot .
USER 10000
CMD ["/pure-bot"]