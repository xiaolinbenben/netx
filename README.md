# NetX 企业办公网络解决方案

本仓库包含 NetX 的独立静态落地页、用户中心应用和管理端。

## 项目结构

- `landing/`：静态落地页，部署到站点根路径 `/`
- `dashboard/`：独立 Next.js 应用，部署到 `/dashboard`
- `admin/`：管理端前端（Vue3 + Vite + Element Plus，基于 pure-admin-thin 裁剪），构建产物由 `server/` 打进 Go 二进制，部署到 `/admin`
- `server/`：Go 后端（标准库 + SQLite），提供 `/api` 接口与 `/admin` 静态资源
- `CNAME`：当前域名记录 `netx.beisi.tech`

落地页不参与 dashboard 的 Next.js 构建。生产环境建议使用 Nginx 或同类反向代理：根路径直接提供 `landing/`，`/dashboard` 及其资源、API 转发到 Next.js 服务。Next.js 页面入口位于 `dashboard/app/page.tsx`，由 `basePath` 映射到外部的 `/dashboard`。

## Dashboard 本地开发

```bash
cd dashboard
npm install
npm run dev
```

开发服务启动后访问 `http://localhost:3000/dashboard`。

健康检查接口为 `http://localhost:3000/dashboard/api/health`。

## Dashboard 生产运行

```bash
cd dashboard
npm install
npm run build
npm run start
```

Next.js 服务默认监听 `3000` 端口。反向代理应保留 `/dashboard` 前缀，不要在转发时剥离路径。

## Docker 部署与 CI/CD

生产环境使用三个独立容器，镜像统一发布到 Docker Hub 的同一个 `netx` 仓库：

- `netx:main-<版本>`：Nginx、落地页和内部反向代理（唯一绑定宿主机端口的容器）
- `netx:dashboard-<版本>`：Dashboard Next.js 服务
- `netx:server-<版本>`：Go API 和内嵌的 Admin

推送 `main` 后，[GitHub Actions](.github/workflows/deploy.yml) 会构建 `linux/amd64` 镜像，推送 `latest` 和 Git commit SHA 两种标签，再通过 SSH 将 [Compose 配置](deploy/docker-compose.yml) 部署到服务器的 `/opt/netx/deploy`。

Docker Hub 需要新建一个名为 `netx` 的公共仓库，并创建具有 Read & Write 权限的 Access Token 用于 GitHub Actions 推送镜像。服务器拉取公共镜像不需要 Docker Hub 登录。GitHub 仓库的 Settings -> Secrets and variables -> Actions 中配置：

| Secret | 说明 |
|---|---|
| `DOCKERHUB_USERNAME` | Docker Hub 用户名 |
| `DOCKERHUB_TOKEN` | Docker Hub Access Token，用于 GitHub Actions 推送镜像，不使用账号密码 |

再创建名为 `production` 的 GitHub Environment，并在其中配置部署 Secrets：

| Secret | 说明 |
|---|---|
| `SERVER_HOST` | Linux 服务器域名或 IP |
| `SERVER_USER` | 可运行 Docker 的 SSH 用户，不配置默认为 `root` |
| `SERVER_SSH_KEY` | SSH 私钥全文 |

服务器首次部署前执行一次初始化，并编辑生产配置：

```bash
sudo mkdir -p /opt/netx/deploy
sudo chown -R "$USER":"$USER" /opt/netx
cd /opt/netx/deploy
# 首次推送 main 后，CI 会上传这份模板
cp .env.example .env
chmod 600 .env
vim .env
```

服务器需安装 Docker Engine 和 Compose 插件，部署用户需有权限直接执行 `docker`。只有 Nginx 容器绑定宿主机的 `8000` 端口，Dashboard 和 Go 服务不发布任何宿主机端口，只能通过内部 Docker 网络访问。宿主机现有的 HTTPS 反向代理应转发到宿主机的 `8000` 端口。SQLite 数据保存在 Docker named volume `netx_server-data` 中，更新容器不会删除数据。

## 管理端与后端

管理端是 Go 后端的一部分：`admin/` 的构建产物输出到 `server/web/dist`，由 Go 通过 `go:embed` 打进二进制，由同一个进程提供 `/admin` 静态页与 `/api` 接口。dashboard 仍是独立的 Next.js 服务。

后端配置全部来自环境变量：

| 环境变量 | 说明 | 默认值 |
|---|---|---|
| `PORT` | 监听端口 | `8080` |
| `DB_PATH` | SQLite 数据库文件路径 | `./data/netx.db` |
| `ADMIN_USERNAME` | 管理员账号，必填 | 无 |
| `ADMIN_PASSWORD` | 管理员密码，必填 | 无 |
| `JWT_SECRET` | 登录凭证签名密钥，必填 | 无 |
访问令牌有效期在后端源码中固定为 2 小时，不通过环境变量配置；刷新令牌有效期固定为 7 天。

管理端本地开发（需要两个终端）：

```bash
cp server/.env.example server/.env
# 编辑 server/.env，设置管理员账号、密码和固定的 JWT 密钥
make admin-install        # 首次安装管理端依赖（使用 pnpm）
make server-dev           # 终端一：Go 后端，监听 8080
make admin-dev            # 终端二：管理端开发服务
```

`server/.env` 已被 Git 忽略，由 `make server-dev` 加载；直接在 `server/` 运行 `go run .` 时仍需自行设置环境变量。生产运行只读取进程环境变量。

浏览器访问 `http://localhost:5173/admin/`，接口请求由 Vite 代理到 8080。

生产构建与运行：

```bash
make build                # 先构建管理端，再构建 server/bin/netx-server
cd server && ./bin/netx-server
```

反向代理需要把 `/admin`、`/api` 与 `/sub` 都转发到该进程。

### 支付宝配置

管理端的“系统配置”中填写项目运行根地址，生产环境使用 `https://netx.beisi.tech`。支付宝配置只需要填写 APPID、应用私钥和支付宝公钥；不再填写通知地址、同步跳转地址、默认订阅源地址或沙箱开关。支付使用管理员预先生成的兑换码库存。

支付请求会自动使用以下地址：

- 异步通知：`https://netx.beisi.tech/api/payment/alipay/notify`
- 支付完成跳转：`https://netx.beisi.tech/access/<兑换码>`

支付成功后，系统会校验支付宝 RSA2 签名，消耗下单时按套餐预占的兑换码，并跳转到专属访问页。订阅上游地址由管理员在生成兑换码时逐个填写。

## 落地页内容

---

> 面向个人、小型团队及企业用户，提供共享节点、独享节点、软硬件一体化三种方案，灵活适配不同场景，满足从日常办公到企业级网络需求的访问体验。

---

## 三大企业网络方案

### 方案一：轻量办公，共享接入

#### 方案介绍

快速接入，满足日常办公与基础网络需求

共享优质网络节点资源，无需部署，快速开通使用

#### 核心能力

- 快速接入

- 安全通信

- 免部署开通，降低日常运维成本

#### 适合用户  

- 个人用户

- 小型团队

- 轻量办公需求

#### 价格

##### 个人轻量接入

¥500 起 / 年

- 流量：1000G / 年

- 线路：CN2 精品线路

- 延迟：低至 160ms 

---

### 方案二：企业私有，稳定独享

#### 方案介绍

独立部署，为企业提供稳定专属网络环境

提供独立网络节点，私有独占，保障更稳定的网络体验

#### 核心能力

- 独享服务器节点

- 专属网络资源

- 多设备接入

- 企业内部资源安全访问

#### 适合用户

- 企业团队

- 多办公室协作

- 企业内部系统访问

#### 价格

##### 企业私有部署

¥3000起 / 年

根据：

- 节点数量

- 服务器配置

- 带宽需求

- 企业规模

进行定制。

---

### 方案三：企业专属，软硬一体

#### 方案介绍

定制部署，打造企业级网络环境

结合 NetX 网络系统与企业级硬件设备，为企业打造专属网络解决方案

#### 核心能力

- 软硬件结合，打造企业专属网络

- 企业级设备部署，保障稳定连接

- 统一管理，灵活控制访问权限

- 提供定制化组网服务

#### 适合用户

- 大型企业

- 多分支机构

- 高稳定性网络需求

#### 价格

##### 企业定制方案

¥10000起 / 年

根据：

- 硬件配置

- 网络规模

- 定制需求

进行定制。

---

## 方案对比

|方案|适合企业|部署方式|价格|
|---|---|---|---|
|轻量办公，共享节点|个人用户、小团队|快速接入，共享资源|¥500起 /年|
|企业私有，独享节点|企业团队|独立节点，稳定可控|¥3000起 /年|
|企业专属，软硬一体|中大型企业|软硬结合，定制部署|¥10000起 /年|

---

## 开始搭建企业专属网络

联系我们
