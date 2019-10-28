FROM golang:1.13-stretch as gobuilder
RUN DEBIAN_FRONTEND=noninteractive apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates
RUN go get -u golang.org/x/lint/golint

RUN addgroup --gid 990 app && adduser --disabled-password --uid 991 --gid 990 --gecos '' app

RUN mkdir -p /build
WORKDIR /build

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -i -v -o release/ipvsmesh -ldflags="-w -s -X main.Version=0.0.1" ipvsmesh.go

FROM debian:stretch
COPY --from=gobuilder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=gobuilder /etc/passwd /etc/passwd
COPY --chown=990:990 --from=gobuilder /build/release/ipvsmesh /app

USER 990:990
ENTRYPOINT ["/app"]
