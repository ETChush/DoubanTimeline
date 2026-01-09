#!/bin/bash

# Docker 重新部署脚本
# 用于快速重新部署更新后的应用

set -e

echo "======================================"
echo "  豆瓣时间线 Docker 重新部署"
echo "======================================"

# 1. 停止并删除旧容器
echo ""
echo "[1/4] 停止并删除旧容器..."
docker-compose down || true

# 2. 删除旧镜像
echo ""
echo "[2/4] 删除旧镜像..."
docker rmi douban-jav-app:latest 2>/dev/null || true

# 3. 重新构建镜像
echo ""
echo "[3/4] 重新构建镜像（包含 Python 环境）..."
docker-compose build --no-cache

# 4. 启动新容器
echo ""
echo "[4/4] 启动新容器..."
docker-compose up -d

# 5. 等待服务启动
echo ""
echo "等待服务启动..."
sleep 5

# 6. 显示容器状态
echo ""
echo "======================================"
echo "  部署完成！"
echo "======================================"
echo ""
docker-compose ps
echo ""

# 7. 显示日志
echo "查看实时日志（Ctrl+C 退出）:"
echo "docker-compose logs -f"
echo ""

# 8. 测试 Python 环境
echo "测试 Python 环境:"
docker-compose exec app python3 --version || echo "警告: Python 测试失败"
echo ""

echo "✅ 部署成功！访问 http://localhost:8080"
echo ""
echo "常用命令："
echo "  查看日志: docker-compose logs -f"
echo "  重启服务: docker-compose restart"
echo "  停止服务: docker-compose down"
