# Caddy 本地开发使用真实域名 HTTPS 方案

## 核心结论

- **根本原因**：Caddy 在非标准端口（8443）运行时，Let's Encrypt HTTP-01 challenge 无法回调，自动 ACME 流程静默失败，HTTPS 握手挂起
- **解决方案**：用 acme.sh DNS-01 challenge 提前签好 `*.yeanhua.asia` 通配符证书，挂入 Caddy 容器，改为手动指定 `tls` 路径
- **DNS 解析**：本地额外需要 `/etc/hosts` 将子域名指向 `127.0.0.1`（ECS 生产环境只需阿里云 DNS A 记录）
- **端口说明**：本地访问带端口 `:8443`；ECS 生产端口改回 `80:80 / 443:443` 后无需端口号

## 问题分析

### 为什么 Let's Encrypt 自动 ACME 不工作

Caddy 默认走 HTTP-01 challenge 流程：

```
Let's Encrypt → GET http://your-domain/.well-known/acme-challenge/xxx → 80 端口
```

但本地 docker-compose 映射的是 `8180:80`，Let's Encrypt 无法从外网访问 `caddy-admin.yeanhua.asia:80`，challenge 超时失败。

即使加了 `/etc/hosts`，DNS 解析正确了，TLS 握手仍然失败（Caddy 没有有效证书）。

### DNS-01 challenge 为什么可以绕开

```
acme.sh → 阿里云 DNS API → 添加 _acme-challenge TXT 记录 → Let's Encrypt 验证 TXT → 签发证书
```

整个过程不需要开放任何 HTTP/HTTPS 端口，完全通过 DNS 验证。

## 解决方案实现

### Caddyfile 修改：改为手动指定证书

```caddyfile
caddy-admin.yeanhua.asia {
    tls /etc/caddy/certs/fullchain.pem /etc/caddy/certs/key.pem

    encode gzip
    # ... 其他配置
}
```

每个站点块都加同一行 `tls`（通配符证书覆盖所有子域名）。

### docker-compose.yml 修改：挂载证书目录

```yaml
caddy:
  volumes:
    - ~/certs/yeanhua.asia:/etc/caddy/certs:ro   # 统一证书目录，按域名区分
    - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro
    # ... 其他 volumes
```

### 证书目录结构（统一 home 目录，按域名区分）

```
~/.acme.sh/                  # acme.sh 工作目录（账户信息、证书元数据），所有项目共享
    ├── account.conf
    └── *.yeanhua.asia_ecc/

~/certs/                     # install-cert 导出的可用证书，按域名分目录
    └── yeanhua.asia/
        ├── fullchain.pem
        └── key.pem
```

## 本地 vs 生产对比

| 项目 | 本地开发 | ECS 生产 |
|------|---------|---------|
| 端口 | `8443:443` | `443:443` |
| 证书 | acme.sh 提前签好，手动挂入 | 同左，或 Caddy 自动 ACME（标准 443） |
| DNS | `/etc/hosts` 指向 `127.0.0.1` | 阿里云 DNS A 记录指向 ECS IP |
| 访问 URL | `https://caddy-admin.yeanhua.asia:8443` | `https://caddy-admin.yeanhua.asia` |

## 证书续签

Let's Encrypt 有效期 90 天，acme.sh 在到期前 30 天自动续签（daemon 模式）。

手动续签流程：

```bash
# 1. 续签
docker run --rm -it -v "$HOME/.acme.sh:/acme.sh" neilpang/acme.sh --cron

# 2. 重新 install-cert
docker run --rm -it \
  -v "$HOME/.acme.sh:/acme.sh" \
  -v "$HOME/certs/yeanhua.asia:/certs" \
  neilpang/acme.sh \
  --install-cert -d "*.yeanhua.asia" \
  --fullchain-file /certs/fullchain.pem \
  --key-file       /certs/key.pem

# 3. 让 Caddy 重新加载证书
docker compose restart caddy
```

## 参考

- acme.sh 详细用法：`https-toolkit/docs/acme-sh-docker-aliyun-dns-2026-02-18.md`
