FROM node:22-alpine AS admin-builder

WORKDIR /src/admin
RUN corepack enable && corepack prepare pnpm@10.18.3 --activate
COPY admin/package.json admin/pnpm-lock.yaml admin/pnpm-workspace.yaml admin/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY admin/ ./
RUN pnpm typecheck && pnpm build

FROM golang:1.26.1-alpine AS server-builder

WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=admin-builder /src/server/web/dist ./web/dist
RUN go test ./... && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/netx-server .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 netx \
    && adduser -S -D -H -u 10001 -G netx netx \
    && mkdir -p /data \
    && chown netx:netx /data
COPY --from=server-builder /out/netx-server /usr/local/bin/netx-server

USER netx
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["netx-server"]
