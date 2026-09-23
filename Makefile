.PHONY: admin-install admin-dev admin-build server-dev server-build test build

# 安装管理端依赖
admin-install:
	cd admin && pnpm install

# 管理端开发服务（http://localhost:5173/admin/，需要同时启动 server-dev）
admin-dev:
	cd admin && pnpm dev

# 构建管理端，产物输出到 server/web/dist（构建会清空目录，补回占位文件保证 go build 可用）
admin-build:
	cd admin && pnpm build
	@touch server/web/dist/.gitkeep

# 后端开发服务（http://localhost:8080）
server-dev:
	cd server && set -a && . ./.env && set +a && go run .

# 后端测试
test:
	cd server && go test ./...

# 构建包含管理端静态资源的后端单文件
server-build: admin-build
	cd server && go build -o bin/netx-server .

# 完整构建
build: server-build
