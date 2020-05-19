FROM golang:1.14.3
WORKDIR /go/src/github.com/juniorz/edgemax-exporter
COPY ./ /go/src/github.com/juniorz/edgemax-exporter
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo .


FROM alpine:latest  
RUN apk --no-cache add ca-certificates
COPY --from=0 /go/src/github.com/juniorz/edgemax-exporter/edgemax-exporter /usr/local/bin/edgemax-exporter

EXPOSE 9745

ENTRYPOINT ["/usr/local/bin/edgemax-exporter"]