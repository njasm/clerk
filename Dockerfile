##
## Build
##
FROM golang:1.18.2-bullseye AS build

ENV GOPATH /go
WORKDIR /go/clerk

# Copy everything and keep sub-folder structures also
COPY ./ ./

RUN go mod download && go build -o /clerk cmd/clerk/main.go

##
## Deploy
##
FROM gcr.io/distroless/base-debian11

WORKDIR /

COPY --from=build /clerk /clerk

#USER nonroot:nonroot

ENTRYPOINT ["/clerk"]
