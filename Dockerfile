ARG ALPINE_VERSION=3.16
FROM alpine:$ALPINE_VERSION
RUN apk add --update-cache git openssh-client \
 && git --version
COPY assets_linux_amd64/check /opt/resource/check
COPY assets_linux_amd64/in /opt/resource/in
COPY assets_linux_amd64/out /opt/resource/out
