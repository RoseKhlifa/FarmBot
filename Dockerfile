# P7-03: single-process production image. The frontend is compiled first and
# copied into the Go embed tree before the static binary is linked.
FROM node:20-alpine AS web-build
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json web/package.json
RUN corepack enable && pnpm install --frozen-lockfile
COPY web web
RUN pnpm -C web build

FROM golang:1.23-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /src/web/dist /src/assets/webdist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/farmbot ./cmd/farmbot

FROM alpine:3.20
RUN addgroup -S farmbot && adduser -S -G farmbot farmbot \
    && apk add --no-cache wget
WORKDIR /app
COPY --from=go-build /out/farmbot /app/farmbot
RUN mkdir -p /app/data && chown -R farmbot:farmbot /app
USER farmbot
EXPOSE 3007
VOLUME ["/app/data"]
ENTRYPOINT ["/app/farmbot"]
