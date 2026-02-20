# 动态服务注册架构实现

## 核心结论

- caddy-admin 升级为「服务注册中心」，新项目通过 `POST /api/services` 自注册路由到 Caddy
- 持久化到 `services.json`，caddy-admin-api 启动时自动 sync 恢复路由（Caddy 重启安全）
- project-c 作为端到端验证：sidecar 容器注册后退出，frontend(nginx) + backend(Go) 通过 Caddy 动态路由对外服务
- 网络通过命名网络 `caddy-net` 打通，外部项目使用 `external: true` 加入

## 架构

```
project-c (外部项目)
  ├─ project-c-frontend (nginx:80)     ← HTML + 反代 /api/* → backend
  ├─ project-c-backend  (:8080)        ← Go API
  └─ project-c-register (curl sidecar) ← POST caddy-admin-api:8090/api/services

caddy-admin (基础设施)
  ├─ caddy          (:443, :80, :2019) ← Caddyfile 引导 + 动态路由
  ├─ caddy-admin-api(:8090)            ← 注册 API + 启动 sync
  ├─ site-a-api / site-b-api           ← 保持不变（Caddyfile 静态路由）
```

## 新增 API

| Method | Path | 说明 |
|--------|------|------|
| `POST` | `/api/services` | 注册/更新服务 → Caddy upsert + 持久化 |
| `DELETE` | `/api/services/{name}` | 注销服务 → Caddy 删除路由 + 持久化 |
| `GET` | `/api/services` | 列出所有已注册服务 |
| `POST` | `/api/services/sync` | 手动触发从 services.json 同步到 Caddy |

## 关键设计决策

### Caddy 路由注入策略
- 使用 `PUT /config/apps/http/servers/srv0/routes/0` prepend 到路由列表头部
- 每条路由带 `@id: "svc-{name}"`，支持 `DELETE /id/svc-{name}` 精确删除
- Caddyfile 末尾添加 `*.yeanhua.asia` 通配符 catch-all（404），动态路由在其之前匹配

### 持久化（FileStore）
- write-tmp-then-rename 原子写入，避免部分写入损坏
- `sync.RWMutex` 保护并发访问
- 内部 `unsafeLoad/unsafeSave` 不加锁，公开方法持有锁期间完成完整操作

### 启动同步
- `syncToCaddy()` 在 goroutine 中运行，重试 15 次 x 2s 等待 Caddy 就绪
- `depends_on` 只保证容器启动，不保证端口就绪

### 网络互通
- `caddy-net` 改为命名网络（`name: caddy-net`）
- project-c 使用 `external: true` 加入同一网络

## 文件变更清单

**修改（4 个）：**
- `caddy-admin/backend/caddy/client.go` — 新增 AddRoute/RemoveRoute/UpsertRoute/do
- `caddy-admin/backend/main.go` — 接线 ServicesHandler + syncToCaddy + CORS 扩展
- `docker-compose.yml` — 命名网络 + services_data volume + SERVICES_FILE env
- `caddy/Caddyfile` — 末尾追加 `*.yeanhua.asia` catch-all

**新建（caddy-admin 内，3 个）：**
- `caddy-admin/backend/caddy/route_builder.go` — ServiceConfig + BuildCaddyRoute
- `caddy-admin/backend/store/file_store.go` — JSON 文件持久化层
- `caddy-admin/backend/handlers/services.go` — 注册/注销/列表/同步 handlers

**新建（project-c，9 个）：**
- `project-c/backend/main.go` + `Dockerfile`
- `project-c/frontend/index.html` + `nginx.conf` + `Dockerfile`
- `project-c/register.sh` + `docker-compose.yml` + `.env` + `Makefile`
