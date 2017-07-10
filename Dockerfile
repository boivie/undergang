FROM golang:1.8 AS builder

WORKDIR /go/src/github.com/boivie/undergang
COPY app  /go/src/github.com/boivie/undergang/app
COPY main.go  /go/src/github.com/boivie/undergang/

RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo .

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/src/github.com/boivie/undergang/undergang .

VOLUME ["/config"]
CMD ["/root/undergang"]
