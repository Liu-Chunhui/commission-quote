FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -o /quotevendor ./cmd

FROM alpine:3.23
WORKDIR /app/quotevendor
COPY --from=build /quotevendor /usr/local/bin/quotevendor
ENTRYPOINT ["quotevendor"]
