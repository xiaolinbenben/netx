# NetX 管理端

基于 [pure-admin-thin](https://github.com/pure-admin/pure-admin-thin)（Vue3 + Vite + Element Plus）裁剪而来，只保留登录、兑换码、系统配置三个页面，接口全部由 `../server` 的 Go 后端提供。

## 开发

```bash
pnpm install
pnpm dev
```

访问 `http://localhost:5173/admin/`。开发服务会把 `/api` 代理到 `http://localhost:8080`，需要先启动 Go 后端。

## 构建

```bash
pnpm build
```

产物输出到 `../server/web/dist`，由 Go 通过 `go:embed` 打进二进制。类型检查用 `pnpm typecheck`。
