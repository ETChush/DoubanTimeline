# DoubanTimeline

一个简单易用的豆瓣时间轴应用，帮助你记录和展示你在豆瓣上看过的电影、书籍和剧集。

## 功能特性

- 📽️ **支持多种媒体类型**：电影、书籍、剧集
- 📅 **时间轴展示**：按年月分组展示你的观看/阅读记录
- 🔗 **一键添加**：通过豆瓣链接快速添加条目
- 🖼️ **本地图片缓存**：自动下载和缓存封面图片
- 🐳 **Docker支持**：提供Docker镜像，方便部署

## 应用截图

### PC端界面
![PC端界面](DoubanTimeline%20Screenshots/PC.png)

### 移动端界面
![移动端界面](DoubanTimeline%20Screenshots/Mobile.png)

## 技术栈

- **后端**：Go语言
- **数据库**：SQLite
- **ORM**：GORM
- **前端**：HTML、CSS、Go templates

## 快速开始

## Docker部署

### 使用Docker Compose（推荐）

1. 克隆仓库：
   ```bash
   git clone https://github.com/ETChush/DoubanTimeline.git
   cd DoubanTimeline
   ```
2. 启动服务：
   ```bash
   docker-compose up -d
   ```
3. 打开浏览器访问 `http://localhost:8080`


## 环境变量

- `DATA_DIR`：数据目录路径，默认值：`.`
- `IMAGE_DIR`：图片存储目录路径，默认值：`images`


## 数据存储

- 数据存储在SQLite数据库文件 `douban_timeline.db` 中
- 封面图片缓存存储在 `images` 目录中

## 许可证

[MIT License](LICENSE)



