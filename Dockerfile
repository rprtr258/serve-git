FROM golang:alpine as build
WORKDIR /build
COPY . /build
RUN go mod tidy
RUN go build -o ./main .

FROM alpine:latest
COPY --from=build /build/main /bin/
ENTRYPOINT ["/bin/main"]
