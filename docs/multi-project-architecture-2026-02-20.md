# 多项目架构决策：单仓库 vs 拆分仓库

> 日期：2026-02-20
> 背景：当前 caddy-admin 项目已包含 caddy + caddy-admin + site-a + site-b，需决定新增前后端项目时的组织方式

## 核心结论

- **< 5 个项目、个人学习**：单仓库（方案 A），简单直接
- **长期维护、项目持续增加**：拆分为「基础设施仓库 + 独立项目仓库」（方案 B），通过 Docker external network 连接
- **深度利用 Caddy Admin API**：方案 B + 动态路由注册（方案 B+），新项目无需修改 Caddyfile

---

## 当前架构

```
caddy-admin/          (git repo root)
├── caddy/Caddyfile   ← 单 Caddy 实例，管理所有 *.yeanhua.asia 路由
├── caddy-admin/      ← Caddy 管理面板 (frontend + backend)
├── site-a/           ← 模拟站点 A (static + api)
├── site-b/           ← 模拟站点 B (static + api)
├── docker-compose.yml← 单一 compose 编排所有服务
└── Makefile
```

特点：
- 单 Caddy 实例反代所有子域名
- 通配符 TLS 证书 `*.yeanhua.asia`
- Admin API 已开启 (`admin 0.0.0.0:2019`)
- 所有服务共享 `caddy-net` Docker 网络

---

## 方案 A：单仓库（继续追加）

### 结构

```
caddy-admin/
├── caddy/Caddyfile          ← 每加一个项目，改这里
├── docker-compose.yml       ← 每加一个项目，加 service
├── caddy-admin/
├── site-a/
├── site-b/
├── new-project-c/           ← 新增
└── new-project-d/           ← 新增
```

### 优劣

| 优势 | 劣势 |
|------|------|
| 一条 `docker compose up` 全启 | 仓库越来越臃肿 |
| Caddyfile 和 compose 集中管理 | 改一个项目的代码，git history 污染其他项目 |
| 本地开发简单，无需协调 | CI/CD 无法独立——改 site-a 会触发全量构建 |
| 适合 1 人、< 5 个小项目 | 多人协作时冲突概率高（都改 compose） |

### 添加新项目步骤

1. 创建 `new-project/frontend/` + `new-project/api/`
2. `docker-compose.yml` 添加 service
3. `caddy/Caddyfile` 添加子域名路由
4. Caddy volumes 挂载前端静态文件

---

## 方案 B：拆分仓库 + Docker External Network

### 结构

```
# Repo 1: caddy-infra（当前仓库精简后）
├── caddy/Caddyfile
├── caddy-admin/
├── docker-compose.yml      ← 只含 caddy + caddy-admin
└── Makefile

# Repo 2: project-c（独立仓库）
├── frontend/
├── backend/
├── docker-compose.yml      ← 加入 external network
└── Makefile
```

### 关键连接方式

```yaml
# caddy-infra/docker-compose.yml
networks:
  caddy-net:
    name: caddy-net          # 命名网络

# project-c/docker-compose.yml
services:
  project-c-api:
    build: ./backend
    networks: [caddy-net]
networks:
  caddy-net:
    external: true           # 加入已有网络
```

### 优劣

| 优势 | 劣势 |
|------|------|
| 各项目独立 git 历史、独立 CI/CD | 需要多个 `docker compose up` |
| 加新项目不影响现有项目代码 | 加新项目仍需改 caddy-infra 的 Caddyfile |
| 适合多人协作、> 5 个项目 | 本地开发需 clone 多个仓库 |
| 各项目可用不同技术栈 | 启动顺序有依赖（先启 caddy-infra） |

---

## 方案 B+：拆分仓库 + Caddy Admin API 动态注册

在方案 B 基础上，新项目启动时通过 Admin API 自注册路由，无需修改 Caddyfile。

### 自注册脚本示例

```bash
# project-c 启动后调用
curl -X POST http://caddy:2019/config/apps/http/servers/srv0/routes \
  -H "Content-Type: application/json" \
  -d '{
    "match": [{"host": ["project-c.yeanhua.asia"]}],
    "handle": [{
      "handler": "subroute",
      "routes": [...]
    }]
  }'
```

### 注意事项

- Admin API 配置是**内存态**，Caddy 重启后丢失
- 需要持久化方案：caddy-admin 面板可承担此职责（保存到数据库，启动时重放）
- 或使用 `caddy persist` 命令将运行时配置写回文件

---

## 选型建议

| 场景 | 推荐 |
|------|------|
| 学习 / 个人 playground，项目 < 5 | 方案 A |
| 长期维护、项目持续增加 | 方案 B |
| 想深度利用 caddy-admin 管理面板能力 | 方案 B+ |
