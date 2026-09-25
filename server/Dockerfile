FROM node:22-alpine AS admin-builder

WORKDIR /src/admin
COPY admin/package.json admin/package-lock.json ./
RUN npm ci
COPY admin/ ./
RUN npm run typecheck && npm run build

FROM node:22-alpine AS dashboard-builder

WORKDIR /src/dashboard
COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm ci
COPY dashboard/ ./
RUN npm run typecheck && npm run build

FROM golang:1.26.1-alpine AS server-builder

WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY landing/index.html landing/weixin.jpg ./web/dist/
COPY --from=admin-builder /src/server/web/dist/admin ./web/dist/admin
COPY --from=dashboard-builder /src/server/web/dist/dashboard ./web/dist/dashboard
RUN go test ./... && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/netx-server .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 netx \
    && adduser -S -D -H -u 10001 -G netx netx \
    && mkdir -p /data \
    && chown netx:netx /data
COPY --from=server-builder /out/netx-server /usr/local/bin/netx-server

USER netx
EXPOSE 8000
VOLUME ["/data"]
ENTRYPOINT ["netx-server"]
