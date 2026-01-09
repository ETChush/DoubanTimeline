# Docker 环境 JAV 功能修复指南

## 问题描述
在 Docker 容器中添加 JAV 影片时出现错误：
```
获取影片信息失败: Python 脚本执行失败: exec: "python": executable file not found in $PATH
```

## 快速修复步骤

### 1. 更新代码文件

确保以下文件已更新：
- ✅ `Dockerfile` - 添加了 Python 3 和依赖安装
- ✅ `main.go` - 修改了 Python 命令检测逻辑
- ✅ `javbus_crawler.py` - 支持 JSON 输出和 UTF-8 编码

### 2. 重新部署（推荐使用脚本）

#### 方式 A：使用自动化脚本（最简单）

```bash
chmod +x redeploy.sh
./redeploy.sh
```

#### 方式 B：手动执行命令

```bash
# 1. 停止容器
docker-compose down

# 2. 删除旧镜像
docker rmi douban-jav-app:latest

# 3. 重新构建（包含 Python 环境）
docker-compose build --no-cache

# 4. 启动容器
docker-compose up -d

# 5. 查看日志
docker-compose logs -f
```

### 3. 验证修复

```bash
# 检查 Python 是否安装
docker-compose exec app python3 --version

# 测试爬虫脚本
docker-compose exec app python3 javbus_crawler.py SSIS-001 --json
```

## 关键变更说明

### Dockerfile 变更

**之前**：
```dockerfile
FROM alpine:latest
RUN apk --no-cache add ca-certificates
```

**之后**：
```dockerfile
FROM alpine:latest
RUN apk --no-cache add \
    ca-certificates \
    python3 \
    py3-pip \
    && python3 -m pip install --break-system-packages httpx aiofiles lxml

COPY --from=builder /app/javbus_crawler.py .
```

### Go 代码变更

**之前**：
```go
cmd := exec.Command("python", "javbus_crawler.py", ...)
```

**之后**：
```go
// 自动检测 python3 或 python
pythonCmd := "python"
if _, err := exec.LookPath("python3"); err == nil {
    pythonCmd = "python3"
}
cmd := exec.Command(pythonCmd, "javbus_crawler.py", ...)
```

## 预期结果

修复后，在 Docker 环境中：
- ✅ 可以正常添加 JAV 影片
- ✅ 中文标题显示正常（无乱码）
- ✅ 封面图片自动下载
- ✅ 所有 JAV 功能正常工作

## 镜像大小变化

- **修复前**：约 20-30 MB
- **修复后**：约 120-150 MB（增加了 Python 运行时）

## 故障排查

### 问题 1：构建时 pip 安装失败

**解决方案**：使用国内镜像源
```dockerfile
RUN python3 -m pip install --break-system-packages \
    -i https://pypi.tuna.tsinghua.edu.cn/simple \
    httpx aiofiles lxml
```

### 问题 2：容器启动后 Python 命令不存在

**检查步骤**：
```bash
# 进入容器
docker-compose exec app sh

# 检查 Python
which python3
python3 --version

# 检查 pip 包
python3 -m pip list
```

### 问题 3：添加 JAV 影片仍然失败

**查看详细错误**：
```bash
# 查看容器日志
docker-compose logs -f app

# 手动测试爬虫
docker-compose exec app python3 javbus_crawler.py SSIS-001 --json
```

## 注意事项

1. **重新构建时使用 `--no-cache`**，确保拉取最新的依赖
2. **数据持久化**：确保 volumes 正确挂载，重建容器不会丢失数据
3. **网络要求**：构建时需要访问 PyPI，添加影片时需要访问 javbus.com

## 完成检查清单

部署后请验证：
- [ ] Docker 容器正常运行
- [ ] 豆瓣功能正常（添加/删除/显示）
- [ ] JAV 功能正常（添加/删除/显示）
- [ ] 收藏功能正常
- [ ] 中文显示无乱码
- [ ] 图片正常加载

## 联系支持

如果遇到其他问题，请查看：
- 详细部署文档：`DOCKER_DEPLOYMENT.md`
- JAV 功能说明：`JAV_FEATURE_README.md`
- 项目主文档：`README.md`
