FROM golang:1.9.1-alpine3.6

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh

WORKDIR /go/src/app
COPY . .

RUN go-wrapper download   # "go get -d -v ./..."
RUN go-wrapper install    # "go install -v ./..."

CMD ["go-wrapper", "run"] # ["app"]