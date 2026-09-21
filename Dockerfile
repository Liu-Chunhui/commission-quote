FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -o /app ./cmd/app

FROM alpine:3.23
WORKDIR /app
COPY --from=build /app /usr/local/bin/app
ENTRYPOINT ["app"]
