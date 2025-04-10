FROM golang:latest
MAINTAINER xtaci <daniel820313@gmail.com>
COPY . /go/src/agent
RUN go install agent
ENTRYPOINT ["/go/bin/agent"]
EXPOSE 8009 8009/udp
