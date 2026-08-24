FROM golang:1.26.2
ENV GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .
RUN go build -mod=vendor -o /usr/local/bin/cryosafe ./cmd/cryosafe
EXPOSE 19698
CMD ["/usr/local/bin/cryosafe"]
