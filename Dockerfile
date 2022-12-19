FROM golang:1.19-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download -x

ADD . /app/

RUN go build -o /hstats ./cmd/hstats.go

FROM alpine
WORKDIR /
ENV ZONEINFO=/zoneinfo.zip
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /zoneinfo.zip
COPY --from=builder /hstats /hstats
EXPOSE 8080
ENTRYPOINT ["/hstats"]