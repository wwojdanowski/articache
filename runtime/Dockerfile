FROM golang:1.20-alpine AS builder

WORKDIR /app 

COPY . .

RUN go mod download

RUN go build -o /articache main.go provider.go

FROM alpine:latest

COPY --from=builder /articache /articache

EXPOSE 8080


ENTRYPOINT [ "/articache" ]