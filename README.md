# PuRe Bot
[![CircleCI branch](https://img.shields.io/circleci/project/github/redhat-ipaas/pure-bot/master.svg)](https://circleci.com/gh/redhat-ipaas/pure-bot)
[![license](https://img.shields.io/github/license/redhat-ipaas/pure-bot.svg)](https://raw.githubusercontent.com/redhat-ipaas/pure-bot/master/LICENSE)
[![Docker Automated buil](https://img.shields.io/docker/automated/redhat-ipaas/pure-bot.svg)](https://hub.docker.com/r/rhipaas/pure-bot/)

**Pu**ll **Re**quest Bot enables automated pull request workflows, reacting to input from webhooks and
performing actions as configured.

Currently actions include:

* Labeling with `approved` label on pull request review approval.
* Automerging a PR once it has `approved` label and passes all required status checks.

## Running

```bash
$ pure-bot help run

Runs pure-bot.

Usage:
pure-bot run [flags]

Flags:
    --bind-address string                     Address to bind to
    --bind-port int                           Port to bind to (default 8080)
    --github-integration-id int               GitHub integration ID
    --github-integration-private-key string   GitHub integration private key file
    --tls-cert string                         TLS cert file
    --tls-key string                          TLS key file
    --webhook-secret string                   Secret to validate incoming webhooks

Global Flags:
    --config string     config file (default is $HOME/.pure-bot.yaml)
    --log-level Level   log level (default info)
```

## Building

```bash
$ make
building: bin/amd64/pure-bot

$ make image
building: bin/amd64/pure-bot
Sending build context to Docker daemon 73.18 MB
Step 1/6 : FROM alpine:3.5
---> 88e169ea8f46
Step 2/6 : MAINTAINER Jimmi Dyson <jimmidyson@gmail.com>
---> Using cache
---> 3cd3ad11bf98
Step 3/6 : RUN apk update && apk upgrade && apk add ca-certificates && rm -rf /var/cache/apk
---> Using cache
---> ae9fde8c1cc7
Step 4/6 : ADD bin/amd64/pure-bot /pure-bot
---> 29cbebdf88fd
Removing intermediate container abec733e4481
Step 5/6 : USER 10000
---> Running in c61f53a8a9fe
---> 78549c7310e4
Removing intermediate container c61f53a8a9fe
Step 6/6 : ENTRYPOINT /pure-bot
---> Running in dcc313c83466
---> 9090fd17e37e
Removing intermediate container dcc313c83466
Successfully built 9090fd17e37e
image: rhipaas/pure-bot:3109e57-dirty
```