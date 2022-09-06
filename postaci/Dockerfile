FROM golang:1.19-alpine as build
RUN apk add build-base

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN go test ./...

RUN go build -o server

FROM alpine:latest
EXPOSE 5000
COPY --from=build /app/server .
CMD [ "./server" ]