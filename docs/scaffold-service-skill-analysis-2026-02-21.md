# Plugin 设计分析：`scaffold-service` — 可分发的动态服务脚手架

## 核心结论

- **完全可行**，且 Claude Code plugin 机制天然支持这种"仓库内置 plugin → 他人安装 → 跨项目使用"模式
- **分发方式**：caddy-admin 仓库本身作为 marketplace，plugin 放在仓库子目录 `plugin/`，他人通过 `claude plugin marketplace add jssfy/caddy-admin` 安装
- **安装后效果**：在任意项目目录下运行 `/caddy-admin:scaffold-service my-project`，生成完整可运行的服务目录
- **命名空间**：安装后 skill 名带 plugin 前缀 → `/caddy-admin:scaffold-service`（不会与其他 plugin 冲突）

---

## 1. 分发架构

### 为什么不能用项目级 `.claude/skills/`

| 方式 | 谁能用 | 安装后在哪 | 跨项目？ |
|------|--------|-----------|---------|
| `.claude/skills/` 项目级 | 只有 clone 了 caddy-admin 并 `cd` 进去的人 | 仅当前项目 | 不行 |
| `~/.claude/skills/` 个人全局 | 手动复制到自己 home 目录 | 全局 | 可以，但不可分发 |
| **Plugin（本方案）** | `claude plugin install` 一键安装 | `~/.claude/plugins/cache/` | 全局可用 |

### 仓库即 Marketplace 模式

参考 `planning-with-files` 的做法——仓库根目录放 `marketplace.json`，plugin 代码放子目录：

```
jssfy/caddy-admin (GitHub)
├── .claude-plugin/
│   └── marketplace.json          ← 声明：本仓库是一个 marketplace
├── plugin/                       ← plugin 源码目录
│   ├── .claude-plugin/
│   │   └── plugin.json           ← plugin 元信息
│   ├── skills/
│   │   └── scaffold-service/
│   │       ├── SKILL.md          ← 脚手架 skill
│   │       └── templates/        ← 模板文件（register.sh 等）
│   └── commands/
│       └── scaffold-service.md   ← slash command 入口（可选）
├── caddy-admin/                  ← 现有项目代码（不受影响）
├── docker-compose.yml
└── ...
```

### 安装流程（用户视角）

```bash
# 1. 添加 marketplace（一次性）
claude plugin marketplace add jssfy/caddy-admin

# 2. 安装 plugin
claude plugin install caddy-admin@jssfy/caddy-admin

# 3. 在任意项目中使用
cd ~/my-workspace
/caddy-admin:scaffold-service my-project
```

---

## 2. Plugin 包结构设计

### 2.1 `marketplace.json`（仓库根目录 `.claude-plugin/`）

```json
{
  "name": "jssfy-caddy-admin",
  "owner": {
    "name": "jssfy",
    "url": "https://github.com/jssfy"
  },
  "plugins": [
    {
      "name": "caddy-admin",
      "source": "./plugin",
      "description": "Scaffold and manage dynamic services for caddy-admin reverse proxy platform",
      "version": "1.0.0",
      "repository": "https://github.com/jssfy/caddy-admin",
      "license": "MIT"
    }
  ]
}
```

### 2.2 `plugin.json`（`plugin/.claude-plugin/`）

```json
{
  "name": "caddy-admin",
  "version": "1.0.0",
  "description": "Scaffold dynamic services for caddy-admin. Generates .env, register.sh, docker-compose.yml, Makefile, and basic backend/frontend code for auto-registration with Caddy reverse proxy.",
  "author": {
    "name": "jssfy",
    "url": "https://github.com/jssfy"
  },
  "repository": "https://github.com/jssfy/caddy-admin",
  "license": "MIT",
  "keywords": [
    "caddy",
    "reverse-proxy",
    "docker",
    "scaffold",
    "service-registration",
    "microservices"
  ]
}
```

### 2.3 Skill（`plugin/skills/scaffold-service/SKILL.md`）

```yaml
---
name: scaffold-service
description: >
  为 caddy-admin 动态注册体系生成新项目脚手架。
  当用户说"创建新服务"、"scaffold service"、"接入新项目到 caddy"时使用。
  生成 .env、register.sh、docker-compose.yml、Makefile 以及 backend/frontend 代码。
user-invocable: true
disable-model-invocation: true
argument-hint: <project-name> [--lang go|node|python] [--port 8080]
allowed-tools:
  - Write
  - Bash
  - Read
  - Glob
---
```

Skill body 包含：
1. 参数解析规则（项目名 / 语言 / 端口）
2. 变量派生规则（从项目名推导 domain、upstream、容器名）
3. 各文件的完整模板（内联 code block）
4. 执行步骤指令（创建目录 → 写入文件 → chmod → 提示下一步）

### 2.4 模板文件（`plugin/skills/scaffold-service/templates/`）

`register.sh` 是通用的、不随语言变化，适合放独立文件：

```
templates/
├── register.sh               # 通用注册脚本
├── Makefile                   # 通用 Makefile
├── go/                        # Go 模板
│   ├── main.go
│   └── Dockerfile
├── node/                      # Node 模板
│   ├── index.js
│   ├── package.json
│   └── Dockerfile
└── frontend/                  # 前端模板（通用）
    ├── index.html
    ├── nginx.conf
    └── Dockerfile
```

Skill 指令中通过 `${CLAUDE_PLUGIN_ROOT}` 引用：

```markdown
## 模板文件

- register.sh 模板位于 `${CLAUDE_PLUGIN_ROOT}/skills/scaffold-service/templates/register.sh`
- 读取模板后，将 `PROJECT_NAME` 占位符替换为用户传入的项目名
```

---

## 3. 用户体验设计

### 3.1 调用示例

```bash
# 最简形式（默认 Go 后端）
/caddy-admin:scaffold-service my-project

# 指定 Node.js 后端
/caddy-admin:scaffold-service my-project --lang node

# 指定端口
/caddy-admin:scaffold-service my-project --lang go --port 9090

# 纯静态站（无后端）
/caddy-admin:scaffold-service my-project --lang static
```

### 3.2 执行输出

```
Creating service scaffold for "my-project"...

  Language: Go
  Domain:   my-project.yeanhua.asia
  Upstream: my-project-frontend:80
  Backend:  my-project-backend:8080

Created files:
  my-project/.env
  my-project/register.sh
  my-project/docker-compose.yml
  my-project/Makefile
  my-project/backend/main.go
  my-project/backend/Dockerfile
  my-project/frontend/index.html
  my-project/frontend/nginx.conf
  my-project/frontend/Dockerfile

Next steps:
  1. cd my-project
  2. docker compose up -d --build
  3. docker compose logs my-project-register  # 确认注册成功
  4. open https://my-project.yeanhua.asia
```

### 3.3 变量派生规则

| 变量 | 派生规则 | `my-project` 示例 |
|------|---------|-------------------|
| `SERVICE_NAME` | = 项目名 | `my-project` |
| `SERVICE_DOMAIN` | = `{项目名}.yeanhua.asia` | `my-project.yeanhua.asia` |
| `SERVICE_UPSTREAM` | = `{项目名}-frontend:80` | `my-project-frontend:80` |
| `CADDY_ADMIN_URL` | 固定值 | `http://caddy-admin-api:8090` |
| backend service 名 | = `{项目名}-backend` | `my-project-backend` |
| frontend service 名 | = `{项目名}-frontend` | `my-project-frontend` |
| register service 名 | = `{项目名}-register` | `my-project-register` |

---

## 4. 方案对比

### 4.1 分发方式对比

| 方式 | 安装命令 | 优势 | 劣势 |
|------|---------|------|------|
| **caddy-admin 仓库作为 marketplace**（推荐） | `marketplace add jssfy/caddy-admin` | 与项目同仓库；一处维护；版本同步 | 仓库结构稍复杂 |
| 独立 plugin 仓库 | `marketplace add jssfy/caddy-admin-plugin` | 职责清晰 | 两个仓库需同步维护 |
| npm 包分发 | `npm install -g @jssfy/caddy-scaffold` | 生态成熟 | 需要 npm 账号；与 Claude Code 集成度低 |

**推荐"仓库即 marketplace"**。理由：
- `planning-with-files` 已验证此模式可行
- 模板文件与基础设施代码同仓库，升级 API 后模板同步更新
- 用户 `marketplace add` 一次，后续版本自动更新

### 4.2 模板存储方式对比

| 方式 | 优势 | 劣势 |
|------|------|------|
| **SKILL.md 内联** code block | 一个文件搞定；Claude 直接读取 | 文件会很长（>300 行）；多语言模板更臃肿 |
| **独立模板文件** + `${CLAUDE_PLUGIN_ROOT}` 引用（推荐） | 模板可单独测试；SKILL.md 保持简洁 | 路径引用需要 `CLAUDE_PLUGIN_ROOT` |
| 动态拉取（`!`curl` 预处理） | 始终最新 | 依赖网络；速度慢 |

**推荐独立模板文件**。SKILL.md 专注于指令逻辑（~100 行），模板文件按语言分目录存放。

---

## 5. 扩展规划

### 5.1 Skills 矩阵

| Skill | 类型 | 用途 | 优先级 |
|-------|------|------|--------|
| `scaffold-service` | user-invocable | 生成新项目脚手架 | P0 |
| `deploy-service` | user-invocable | 构建 + 启动 + 验证注册 | P1 |
| `teardown-service` | user-invocable | 注销 + 停止 + 清理 | P1 |
| `caddy-admin-guide` | auto（`user-invocable: false`） | 架构背景知识，Claude 按需自动加载 | P1 |

### 5.2 完整 Plugin 目录（最终形态）

```
plugin/
├── .claude-plugin/
│   └── plugin.json
├── skills/
│   ├── scaffold-service/
│   │   ├── SKILL.md
│   │   └── templates/
│   │       ├── register.sh
│   │       ├── Makefile
│   │       ├── docker-compose.yml.tmpl
│   │       ├── go/
│   │       ├── node/
│   │       └── frontend/
│   ├── deploy-service/
│   │   └── SKILL.md
│   ├── teardown-service/
│   │   └── SKILL.md
│   └── caddy-admin-guide/
│       └── SKILL.md
└── README.md
```

---

## 6. 实施步骤

### Phase 1：创建 Plugin 框架 + scaffold-service skill

```bash
# 1. 仓库根目录创建 marketplace 声明
mkdir -p .claude-plugin
# → .claude-plugin/marketplace.json

# 2. 创建 plugin 目录结构
mkdir -p plugin/.claude-plugin
mkdir -p plugin/skills/scaffold-service/templates/{go,node,frontend}
# → plugin/.claude-plugin/plugin.json
# → plugin/skills/scaffold-service/SKILL.md
# → plugin/skills/scaffold-service/templates/...

# 3. 本地测试
claude --plugin-dir ./plugin
/caddy-admin:scaffold-service test-project
```

### Phase 2：端到端验证

```bash
# 1. 推送到 GitHub
git add plugin/ .claude-plugin/ && git commit && git push

# 2. 模拟他人安装
claude plugin marketplace add jssfy/caddy-admin
claude plugin install caddy-admin@jssfy-caddy-admin

# 3. 在其他项目中使用
cd ~/some-other-project
/caddy-admin:scaffold-service new-service

# 4. 验证生成的项目可运行
cd new-service && docker compose up -d --build
```

### Phase 3：扩展 skills

按 P1 优先级依次添加 `deploy-service`、`teardown-service`、`caddy-admin-guide`。
