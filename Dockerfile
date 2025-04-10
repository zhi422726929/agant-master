FROM golang:latest
WORKDIR /go/src/agent
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agent .
EXPOSE 8009 8009/udp
CMD ["./agent"]
