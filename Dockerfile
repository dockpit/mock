FROM google/golang:1.3

WORKDIR /gopath/src/app
ADD . /gopath/src/app/
ADD ./server /gopath/src/github.com/dockpit/mock/server
ADD ./manager /gopath/src/github.com/dockpit/mock/manager
RUN go get -ldflags "-X main.Version `cat VERSION` -X main.Build `date -u +%Y%m%d%H%M%S`" app

CMD []
EXPOSE 8000
ENTRYPOINT ["/gopath/bin/app"]