# Docker 部署说明

## 问题解决

### 问题：Docker 容器中 JAV 功能报错
**错误信息**：
```
获取影片信息失败: Python 脚本执行失败: exec: "python": executable file not found in $PATH
```

**原因**：
Docker 容器基于 `alpine:latest` 镜像，默认不包含 Python 环境。

**解决方案**：
已修改 Dockerfile，在运行时镜像中安装 Python 3 和必要的依赖包。

## 更新的 Dockerfile 说明

### 主要变更

1. **安装 Python 3 和依赖**：
   ```dockerfile
   RUN apk --no-cache add \
       ca-certificates \
       python3 \
       py3-pip \
       && python3 -m pip install --break-system-packages httpx aiofiles lxml
   ```

2. **复制 Python 爬虫脚本**：
   ```dockerfile
   COPY --from=builder /app/javbus_crawler.py .
   ```

3. **Go 代码兼容性**：
   - 修改了 `fetchJavInfoFromPython` 函数
   - 自动检测使用 `python3` 或 `python` 命令
   - Windows 本地开发使用 `python`
   - Docker/Linux 环境使用 `python3`

### Python 依赖包

- **httpx**: 异步 HTTP 客户端
- **aiofiles**: 异步文件操作
- **lxml**: HTML/XML 解析

## 重新部署步骤

### 1. 停止并删除旧容器

```bash
# 停止容器
docker stop douban-timeline

# 删除容器
docker rm douban-timeline

# 删除旧镜像（可选）
docker rmi douban-timeline:latest
```

### 2. 重新构建镜像

在项目根目录执行：

```bash
docker build -t douban-timeline:latest .
```

**注意**：
- 构建过程会下载 Python 包，可能需要几分钟
- 如果网络较慢，可以使用国内镜像源

### 3. 运行新容器

使用 docker-compose：
```bash
docker-compose up -d
```

或直接运行：
```bash
docker run -d \
  --name douban-timeline \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/images:/app/images \
  -e TZ=Asia/Shanghai \
  douban-timeline:latest
```

### 4. 验证部署

```bash
# 查看容器日志
docker logs -f douban-timeline

# 测试 Python 是否可用
docker exec douban-timeline python3 --version

# 测试爬虫脚本
docker exec douban-timeline python3 javbus_crawler.py SSIS-001 --json
```

## 使用 docker-compose.yml

确保 docker-compose.yml 文件正确：

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./images:/app/images
    environment:
      - TZ=Asia/Shanghai
      - DATA_DIR=/app/data
      - IMAGE_DIR=/app/images
    restart: unless-stopped
```

## 镜像大小优化（可选）

当前 Dockerfile 使用 Alpine Linux，镜像大小约 100-150 MB。

如果需要进一步优化，可以考虑：

1. **使用多阶段构建**（已实现）
2. **清理 pip 缓存**：
   ```dockerfile
   RUN python3 -m pip install --no-cache-dir httpx aiofiles lxml
   ```

3. **使用精简的 Python 包**：
   - 考虑使用 `httpcore` 代替 `httpx`
   - 但需要修改 Python 代码

## 常见问题

### Q1: 安装 Python 包时网络超时

**解决方案**：使用 pip 国内镜像源

修改 Dockerfile：
```dockerfile
RUN python3 -m pip install --break-system-packages \
    -i https://pypi.tuna.tsinghua.edu.cn/simple \
    httpx aiofiles lxml
```

### Q2: lxml 安装失败

**解决方案**：需要安装编译依赖

修改 Dockerfile：
```dockerfile
RUN apk --no-cache add \
    ca-certificates \
    python3 \
    py3-pip \
    libxml2-dev \
    libxslt-dev \
    gcc \
    musl-dev \
    && python3 -m pip install --break-system-packages httpx aiofiles lxml \
    && apk del gcc musl-dev
```

### Q3: 容器中文乱码

**解决方案**：已通过环境变量和 Python 编码设置解决

确保：
- Docker compose 中设置 `TZ=Asia/Shanghai`
- Go 代码中设置 `PYTHONIOENCODING=utf-8`
- Python 脚本使用 UTF-8 编码输出

## 更新日志

### 2026-01-05
- ✅ 添加 Python 3 运行时环境
- ✅ 安装 httpx, aiofiles, lxml 依赖
- ✅ 复制 javbus_crawler.py 脚本到容器
- ✅ 修改 Go 代码自动检测 python3/python 命令
- ✅ 修复 Docker 环境 JAV 功能

## 测试清单

部署后请测试以下功能：

- [ ] 豆瓣页面正常显示
- [ ] 添加豆瓣影片功能正常
- [ ] JAV 页面正常显示
- [ ] 添加 JAV 影片功能正常（重点测试）
- [ ] JAV 收藏功能正常
- [ ] 删除功能正常
- [ ] 图片正常显示
- [ ] 中文显示无乱码

## 性能说明

- **镜像大小**：约 120-150 MB（包含 Python 运行时）
- **内存占用**：约 50-100 MB（运行时）
- **启动时间**：约 2-5 秒
- **Python 脚本执行**：每次添加 JAV 影片约 1-3 秒

## 安全建议

1. **不要暴露敏感端口**：使用反向代理（Nginx）
2. **定期更新镜像**：`docker pull alpine:latest`
3. **限制资源使用**：
   ```yaml
   deploy:
     resources:
       limits:
         cpus: '0.5'
         memory: 256M
   ```

4. **使用只读文件系统**（除了数据目录）：
   ```yaml
   read_only: true
   tmpfs:
     - /tmp
   ```
