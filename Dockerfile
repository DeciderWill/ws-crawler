FROM golang:1.6

WORKDIR /go/src/app
COPY . .

RUN go install -v

CMD ["app"]