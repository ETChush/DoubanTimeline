# DoubanTimeline Docker 部署指南

本文档介绍如何使用 Docker 部署 DoubanTimeline 应用。

## 前置要求

- Docker 20.10+
- Docker Compose 2.0+（可选，但推荐）

## 快速开始

### 方法一：使用 Docker Compose（推荐）

1. **准备目录结构**

```bash
# 创建项目目录
mkdir douban-timeline
cd douban-timeline

# 复制项目文件到当前目录
# （确保包含 Dockerfile, docker-compose.yml, go.mod, main.go 等所有文件）
```

2. **创建数据目录**

```bash
# 创建数据存储目录（用于持久化数据库和图片）
mkdir -p data images
```

3. **启动服务**

```bash
# 使用 docker-compose 启动
docker-compose up -d

# 查看日志
docker-compose logs -f
```

4. **访问应用**

打开浏览器访问：http://localhost:8080

### 方法二：使用 Docker 命令

1. **构建镜像**

```bash
docker build -t douban-timeline:latest .
```

2. **运行容器**

```bash
docker run -d \
  --name douban-timeline \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/images:/app/images \
  -e DATA_DIR=/app/data \
  -e IMAGE_DIR=/app/images \
  -e TZ=Asia/Shanghai \
  douban-timeline:latest
```

3. **查看日志**

```bash
docker logs -f douban-timeline
```

## 数据持久化

### 挂载说明

应用使用以下目录存储数据：

- **`./data`** - 数据库文件存储目录
  - 挂载到容器内的 `/app/data`
  - 包含 `douban_timeline.db` SQLite 数据库文件

- **`./images`** - 电影海报图片存储目录
  - 挂载到容器内的 `/app/images`
  - 所有下载的电影海报都存储在这里

### 备份数据

```bash
# 备份数据库
cp data/douban_timeline.db data/douban_timeline.db.backup

# 备份图片
tar -czf images_backup.tar.gz images/
```

### 恢复数据

```bash
# 恢复数据库
cp data/douban_timeline.db.backup data/douban_timeline.db

# 恢复图片
tar -xzf images_backup.tar.gz
```

## 环境变量

可以通过环境变量配置应用：

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `DATA_DIR` | `.` | 数据库文件存储目录 |
| `IMAGE_DIR` | `images` | 图片存储目录 |
| `TZ` | `Asia/Shanghai` | 时区设置 |

## 常用操作

### 停止服务

```bash
# 使用 docker-compose
docker-compose down

# 或使用 docker 命令
docker stop douban-timeline
docker rm douban-timeline
```

### 重启服务

```bash
# 使用 docker-compose
docker-compose restart

# 或使用 docker 命令
docker restart douban-timeline
```

### 更新应用

```bash
# 1. 停止当前容器
docker-compose down

# 2. 重新构建镜像
docker-compose build

# 3. 启动新容器
docker-compose up -d
```

### 查看容器状态

```bash
docker-compose ps
# 或
docker ps | grep douban-timeline
```

### 进入容器

```bash
docker exec -it douban-timeline sh
```

## 故障排查

### 1. 容器无法启动

检查日志：
```bash
docker-compose logs
# 或
docker logs douban-timeline
```

常见问题：
- 端口 8080 已被占用：修改 `docker-compose.yml` 中的端口映射
- 目录权限问题：确保 `data` 和 `images` 目录有写权限

### 2. 图片无法显示

- 检查 `images` 目录是否正确挂载
- 检查容器内 `/app/images` 目录是否存在
- 查看应用日志确认图片下载是否成功

### 3. 数据库问题

- 检查 `data` 目录是否正确挂载
- 检查数据库文件权限
- 可以删除 `douban_timeline.db` 文件重新初始化（**注意：会丢失所有数据**）

### 4. 网络问题

如果无法访问豆瓣 API：
- 检查容器网络连接
- 检查防火墙设置
- 考虑使用代理（需要修改代码）

## 生产环境建议

1. **使用反向代理**

在生产环境中，建议使用 Nginx 或 Traefik 作为反向代理：

```nginx
# Nginx 配置示例
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

2. **定期备份**

设置定时任务定期备份数据：

```bash
# 添加到 crontab
0 2 * * * /path/to/backup.sh
```

3. **监控和日志**

- 使用 Docker 日志驱动收集日志
- 配置健康检查（docker-compose.yml 中已包含）
- 监控容器资源使用情况

4. **安全建议**

- 不要将应用暴露在公网，使用反向代理
- 定期更新 Docker 镜像
- 限制容器资源使用（CPU、内存）

## 卸载

```bash
# 停止并删除容器
docker-compose down

# 删除镜像（可选）
docker rmi douban-timeline:latest

# 删除数据（谨慎操作！）
# rm -rf data images
```

## 技术支持

如遇到问题，请检查：
1. Docker 和 Docker Compose 版本
2. 系统资源（磁盘空间、内存）
3. 网络连接
4. 应用日志

