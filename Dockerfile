FROM golang:1.10-alpine as build-env
WORKDIR /go/src/github.com/gilgameshskytrooper/GO-Chatroom/
RUN apk --no-cache add ca-certificates && apk --no-cache add git
COPY . ./
RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o GO-Chatroom .

FROM scratch
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-env /go/src/github.com/gilgameshskytrooper/GO-Chatroom/GO-Chatroom /
COPY --from=build-env /go/src/github.com/gilgameshskytrooper/GO-Chatroom/index.html /
CMD ["./GO-Chatroom"]
