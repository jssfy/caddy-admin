# Caddy TLS 证书问题定位

## 核心结论

- 删除 `~/certs/yeanhua.asia/` 下的 pem 文件后，Caddy **仍能正常启动**——因为 `caddy_data` Docker named volume 里有证书缓存
- 同时删除 pem 文件 **和** `caddy_data` volume 后，Caddy **启动失败**，只剩 3 个容器运行
- 证书续签后，需要 `docker compose restart caddy` 让 Caddy 重新加载新证书

---

## 问题现象

### 现象 1：删除证书文件后 Caddy 仍正常

```bash
# 删除证书文件
rm ~/certs/yeanhua.asia/*

# 确认文件已删除
ls ~/certs/yeanhua.asia    # 空目录

# 重启服务
docker compose up -d

# Caddy 仍然正常，日志显示：
# "skipping automatic certificate management because one or more matching certificates are already loaded"
```

**原因**：`caddy_data` 是 Docker named volume，Caddy 首次加载 pem 文件后会缓存到 `/data/caddy/` 内部存储。named volume 在 `docker compose down` + `up` 之间持久存在，所以 Caddy 从 volume 缓存中找到了证书。

### 现象 2：同时删除 volume 后 Caddy 启动失败

```bash
# 删除 caddy_data volume
docker compose down
docker volume rm caddy-admin_caddy_data

# 重启服务
docker compose up -d

# docker ps 只显示 3 个容器，caddy 容器缺失
# caddy 启动后立即退出——找不到 Caddyfile tls 指令指定的 pem 文件
```

## 证书加载优先级

```
1. Caddyfile tls 指令 → 读 /etc/caddy/certs/*.pem（挂载自 ~/certs/yeanhua.asia/）
2. 加载成功后 → Caddy 自动缓存到 /data/caddy/（caddy_data volume）
3. 后续重启 → 优先从 volume 缓存加载，即使原始 pem 文件已不存在
4. 如果 volume 也被清除 → 回退到读 pem 文件 → 文件不存在则启动失败
```

## 修复步骤

```bash
# 1. 重新签发（~/.acme.sh 有缓存时自动跳过，不会重复向 CA 请求）
docker run --rm -it \
  -v "$HOME/.acme.sh:/acme.sh" \
  -e Ali_Key="${Ali_Key}" \
  -e Ali_Secret="${Ali_Secret}" \
  neilpang/acme.sh \
  --issue -d "*.yeanhua.asia" -d yeanhua.asia \
  --dns dns_ali --server letsencrypt

# 2. 安装证书到统一目录
mkdir -p ~/certs/yeanhua.asia
docker run --rm -it \
  -v "$HOME/.acme.sh:/acme.sh" \
  -v "$HOME/certs/yeanhua.asia:/certs" \
  neilpang/acme.sh \
  --install-cert -d "*.yeanhua.asia" \
  --fullchain-file /certs/fullchain.pem \
  --key-file       /certs/key.pem

# 3. 重启 Caddy
docker compose up -d
```

## 证书续签处理

Let's Encrypt 有效期 90 天，续签后需要让 Caddy 重新加载：

```bash
# 续签（acme.sh 自动判断是否需要）
docker run --rm -it -v "$HOME/.acme.sh:/acme.sh" neilpang/acme.sh --cron

# 重新 install-cert（更新 ~/certs/yeanhua.asia/ 下的文件）
docker run --rm -it \
  -v "$HOME/.acme.sh:/acme.sh" \
  -v "$HOME/certs/yeanhua.asia:/certs" \
  neilpang/acme.sh \
  --install-cert -d "*.yeanhua.asia" \
  --fullchain-file /certs/fullchain.pem \
  --key-file       /certs/key.pem

# 让 Caddy 重新读取证书（不影响其他容器）
docker compose restart caddy
```

**注意**：仅 `restart caddy` 即可，不需要 `down` + `up` 全部容器。Caddy 重启时会从 pem 文件重新加载并更新 volume 缓存。

## 关键认知

| 操作 | 影响 |
|------|------|
| 删除 `~/certs/yeanhua.asia/*.pem` | Caddy 仍正常（volume 缓存兜底） |
| `docker volume rm caddy_data` | 清除缓存，下次启动必须有 pem 文件 |
| 两者都删 | Caddy 启动失败 |
| 证书续签后不重启 Caddy | Caddy 继续用旧证书（volume 缓存），直到旧证书过期 |
