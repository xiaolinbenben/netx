.PHONY: admin-install admin-dev admin-build dashboard-install dashboard-dev dashboard-build server-dev server-build test build

# 安装 React + Ant Design Pro 管理端依赖
admin-install:
	cd admin && npm install

# 管理端开发服务（http://localhost:5173/admin/，需要同时启动 server-dev）
admin-dev:
	cd admin && npm run dev

# 构建管理端，产物输出到 server/web/dist/admin
admin-build:
	cd admin && npm run build

# 构建用户端，产物输出到 server/web/dist/dashboard
dashboard-install:
	cd dashboard && npm install

dashboard-dev:
	cd dashboard && npm run dev

dashboard-build:
	cd dashboard && npm run build

# 后端开发服务（http://localhost:8000）
server-dev:
	cd server && set -a && . ./.env && set +a && go run .

# 后端测试
test:
	cd server && go test ./...

# 构建包含管理端静态资源的后端单文件
server-build: admin-build dashboard-build
	cd server && go build -o bin/netx-server .

# 完整构建
build: server-build
