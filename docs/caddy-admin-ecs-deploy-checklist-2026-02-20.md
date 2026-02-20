# ECS 部署验证清单

> 更新时间：2026-02-20
> 域名：*.yeanhua.asia（阿里云 DNS）
> 目标：验证"单台 ECS + Caddy 单进程 + 真实 Let's Encrypt 证书"部署链路

---

## 本地验证（已完成 ✅）

| 验证点 | 结果 |
|--------|------|
| Caddy Admin API 解析站点列表（域名/类型/upstream/CORS）| ✅ 6 个站点全部正确 |
| TLS 证书读取（颁发者/有效期）| ✅ 6 张本地 CA 证书正确读出 |
| Docker Compose 多服务编排 | ✅ 4 个容器正常启动 |
| 前端 SPA build + 静态文件服务 | ✅ |
| caddy-admin-api 通过 Caddy 反代访问 | ✅ |

---

## ECS 阶段验证（待执行）

### 前提条件（首次 SSH 上 ECS 后执行）

```bash
# 检查 Docker
docker --version && docker compose version

# 检查 Caddy
caddy version

# 检查安全组（应已开放 22 / 80 / 443）
curl -s ifconfig.me   # 获取 ECS 公网 IP
```

如果 Docker / Caddy 未安装，执行初始化脚本：
```bash
# 安装 Docker
curl -fsSL https://get.docker.com | sh
systemctl enable docker && systemctl start docker

# 安装 Caddy
apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    | tee /etc/apt/sources.list.d/caddy-stable.list
apt update && apt install -y caddy
systemctl enable caddy
```

---

### 验证 1：DNS A 记录（阿里云控制台或 API）

在阿里云 DNS 控制台添加以下 A 记录（指向 ECS 公网 IP）：

| 主机记录 | 记录类型 | 记录值 |
|----------|---------|--------|
| `caddy-admin` | A | `<ECS_IP>` |
| `api.caddy-admin` | A | `<ECS_IP>` |

验证 DNS 生效：
```bash
dig caddy-admin.yeanhua.asia +short    # 应返回 ECS IP
dig api.caddy-admin.yeanhua.asia +short
```

- [ ] `caddy-admin.yeanhua.asia` → ECS IP ✓
- [ ] `api.caddy-admin.yeanhua.asia` → ECS IP ✓

---

### 验证 2：Caddyfile 配置 + SSL 自动签发

将 `deploy/Caddyfile` 内容追加到 ECS 的 `/etc/caddy/Caddyfile`，然后 reload：

```bash
# ECS 上执行
cat >> /etc/caddy/Caddyfile << 'EOF'

caddy-admin.yeanhua.asia {
    root * /var/www/caddy-admin/dist
    file_server
    try_files {path} /index.html
    encode gzip
}

api.caddy-admin.yeanhua.asia {
    reverse_proxy localhost:8090
    header {
        Access-Control-Allow-Origin "https://caddy-admin.yeanhua.asia"
        Access-Control-Allow-Methods "GET, OPTIONS"
        Access-Control-Allow-Headers "Content-Type"
    }
}
EOF

systemctl reload caddy
```

等待 30–60 秒，Caddy 自动向 Let's Encrypt 申请证书：
```bash
# 查看证书申请日志
journalctl -u caddy -n 30 --no-pager

# 验证证书已签发
curl -v https://caddy-admin.yeanhua.asia 2>&1 | grep "SSL certificate"
```

- [ ] Caddy reload 无报错 ✓
- [ ] 日志中出现 `certificate obtained successfully` ✓
- [ ] `curl -v` 显示 Let's Encrypt 颁发的证书 ✓

---

### 验证 3：部署 caddy-admin 后端

```bash
# 本地执行（上传代码 + 启动）
ECS_IP=<your-ecs-ip> ./deploy/deploy.sh

# 或手动步骤：
# 1. 上传后端代码
scp -r backend/ root@<ECS_IP>:/opt/caddy-admin/

# 2. ECS 上构建并启动
ssh root@<ECS_IP> "
  cd /opt/caddy-admin
  docker compose -f deploy/docker-compose.yml up -d --build
"
```

验证后端启动：
```bash
# ECS 本地测试（不走域名）
ssh root@<ECS_IP> "curl -s http://localhost:8090/api/status"
# 期望：{"caddy":true}

ssh root@<ECS_IP> "curl -s http://localhost:8090/api/sites | python3 -m json.tool | head -30"
# 期望：包含 caddy-admin.yeanhua.asia 站点

ssh root@<ECS_IP> "curl -s http://localhost:8090/api/certs | python3 -m json.tool"
# 期望：包含 Let's Encrypt 签发的 caddy-admin.yeanhua.asia 证书
```

- [ ] `api/status` → `caddy: true` ✓
- [ ] `api/sites` 包含 yeanhua.asia 域名 ✓
- [ ] `api/certs` 显示 Let's Encrypt 证书，source: letsencrypt ✓

---

### 验证 4：部署前端 + 公网 HTTPS 访问

```bash
# 本地执行：构建前端并上传
cd frontend && npm run build && cd ..
rsync -avz frontend/dist/ root@<ECS_IP>:/var/www/caddy-admin/dist/
```

公网验证：
```bash
# 从任意机器执行
curl -sf https://caddy-admin.yeanhua.asia && echo "✅ 前端 HTTPS 正常"
curl -sf https://api.caddy-admin.yeanhua.asia/api/status && echo "✅ API HTTPS 正常"
curl -sf https://api.caddy-admin.yeanhua.asia/api/sites | python3 -m json.tool
curl -sf https://api.caddy-admin.yeanhua.asia/api/certs | python3 -m json.tool
```

浏览器访问 `https://caddy-admin.yeanhua.asia` 确认：
- [ ] HTTPS 绿锁，证书颁发者为 Let's Encrypt ✓
- [ ] Sites 页面正确显示站点列表 ✓
- [ ] Certs 页面显示真实证书（source: letsencrypt，有效期 90 天）✓
- [ ] 点击站点进入详情页，SPA 路由正常（刷新不 404）✓

---

### 验证 5：Caddy 单进程管理多项目可行性

在 Caddyfile 再追加一个模拟项目，reload，确认不影响已有站点：

```bash
cat >> /etc/caddy/Caddyfile << 'EOF'

# 新项目接入测试
test-site.yeanhua.asia {
    respond "Hello from test-site" 200
}
EOF

systemctl reload caddy
curl -sf https://test-site.yeanhua.asia   # 新站点可访问
curl -sf https://caddy-admin.yeanhua.asia  # 旧站点不受影响
```

- [ ] 新站点 test-site 在不重启的情况下接入成功 ✓
- [ ] caddy-admin 和其他已有站点正常 ✓
- [ ] caddy-admin 面板中 Sites 列表自动更新包含 test-site ✓

---

## 验证结论模板（完成后填写）

```
ECS 验证日期：____
ECS 规格：____
Caddy 版本：____

1. DNS A 记录：✅ / ❌
2. SSL 自动签发：✅ / ❌（耗时：__秒）
3. caddy-admin 后端 HTTPS：✅ / ❌
4. 前端 SPA HTTPS：✅ / ❌
5. 多项目热接入（无重启）：✅ / ❌

遇到的问题：
-

部署方案可行性结论：
```

---

## 注意事项

- **Let's Encrypt 频率限制**：同一域名 1 周内最多失败 5 次，测试时注意不要反复触发失败的 ACME 请求
- **端口 80 必须开放**：HTTP-01 验证需要，安全组入方向必须有 TCP:80 规则
- **证书路径**（系统 Caddy，非 Docker）：`/var/lib/caddy/.local/share/caddy/certificates/`
  - 与本地 Docker 版不同，`deploy/docker-compose.yml` 中的 volume 路径需对应调整
  - 实际挂载后在容器内查看：`find /data -name "*.crt"`
