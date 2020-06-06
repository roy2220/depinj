FROM golang:1.13.8-alpine3.11

VOLUME /project

ENV GOPROXY=https://mirrors.aliyun.com/goproxy/

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories \
    && apk add -q --no-progress --no-cache bash \
                                           make

ADD go.mod go.sum /project/
WORKDIR /project
RUN go mod download
