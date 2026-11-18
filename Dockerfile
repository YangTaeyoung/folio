FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist/ internal/webassets/dist/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /folio ./cmd/folio

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 folio && adduser -D -u 10001 -G folio folio && mkdir /data && chown folio:folio /data
COPY --from=build /folio /usr/local/bin/folio
USER folio
WORKDIR /data
EXPOSE 4310
VOLUME ["/data"]
ENTRYPOINT ["folio"]
CMD ["serve", "--listen", "0.0.0.0:4310", "--data-dir", "/data"]
