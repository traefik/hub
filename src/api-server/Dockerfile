# syntax=docker/dockerfile:1.8

# golang builder
FROM golang:1.22 as builder
WORKDIR /usr/local/src/
COPY . /usr/local/src/
RUN make build

# docker image
FROM alpine

RUN apk --no-cache --no-progress add ca-certificates tzdata \
    && rm -rf /var/cache/apk/*

ARG TARGETPLATFORM
COPY --from=builder /usr/local/src/dist/$TARGETPLATFORM/api-server /

ENTRYPOINT ["/api-server"]
EXPOSE 3000
