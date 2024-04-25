FROM golang:1.19-bookworm

RUN apt update && apt install -y libudev-dev libusb-1.0-0 libusb-1.0-0-dev

WORKDIR /app
COPY ./pkg ./go.mod ./go.sum /app/

# RUN go mod tidy

ENTRYPOINT []
