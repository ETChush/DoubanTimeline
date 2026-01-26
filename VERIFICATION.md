# 验证脚本 - 检查修复是否生效

## 问题1：断连错误日志刷屏
### 测试方法
1. 启动服务：`go run main.go`
2. 在浏览器中快速刷新页面或切换标签页
3. 观察日志输出

### 预期结果
- ❌ 不应该看到 "broken pipe" 或 "connection reset" 错误
- ✅ 只应该看到真正的模板执行错误（如果有）

---

## 问题2：豆瓣评分抓取
### 测试方法
1. 启动服务
2. 在表单中输入豆瓣链接：`https://movie.douban.com/subject/22735748/`
3. 点击添加

### 预期结果
- ✅ 应该看到 API 返回评分（优先）
- ✅ 如果 API 没有评分，应该从 HTML 页面抓取
- ✅ 日志中应该显示以下内容之一：

#### 情况1：API返回评分
```
[addMovie] 豆瓣ID 22735748 不存在，准备创建新记录
[addMovie] ✓ API成功返回评分: 9.7
```

#### 情况2：HTML抓取评分
```
[addMovie] 豆瓣ID 22735748 不存在，准备创建新记录
[addMovie] API未返回评分，尝试从HTML页面抓取，豆瓣链接: https://movie.douban.com/subject/22735748/
[fetchRating] ========== 开始抓取豆瓣评分 ==========
[fetchRating] 请求URL: https://movie.douban.com/subject/22735748/
[fetchRating] 发送HTTP请求...
[fetchRating] ✓ 最终响应状态码: 200
[fetchRating] ✓ 成功读取响应体，大小: xxxxx bytes
[fetchRating] ✓ 抓取成功，评分: 9.7
[fetchRating] ========== 评分抓取结束 ==========
```

---

## 修复内容总结

### douban_module/douban.go
1. ✅ 添加 `DoubanRating` 结构体（包含评分信息）
2. ✅ 在 `DoubanMovieSubject`、`DoubanBookSubject`、`DoubanGameSubject` 中添加 `Rating` 字段
3. ✅ 在 `MediaSubject` 中添加 `Rating` 字段
4. ✅ 在 `FetchDoubanMediaInfo` 中提取并返回评分

### main.go
1. ✅ 修改 `fetchRating` 函数：移除 `CheckRedirect` 限制，允许跟随302重定向
2. ✅ 修改 `addMovieHandler`：优先使用 API 返回的评分，作为备用才调用 `fetchRating`
3. ✅ 优化 `fetchRating` 日志输出：使用图标和分隔线，便于追踪

---

## 为什么这样做？

1. **优先使用API**：豆瓣API返回的数据更可靠，不易被反爬机制拦截
2. **HTML抓取作为备用**：如果API没有返回评分，仍然可以从HTML页面抓取
3. **允许重定向**：豆瓣页面可能需要重定向才能访问，禁止重定向会导致302错误
4. **双重保障**：API + HTML抓取，确保评分获取成功率高

---

## 关键修改点

| 文件 | 行号 | 修改内容 |
|------|------|----------|
| douban.go | 29-33 | 添加 `DoubanRating` 结构体 |
| douban.go | 45-54 | 在各Subject中添加 `Rating` 字段 |
| douban.go | 68-77 | 在 `MediaSubject` 中添加 `Rating` 字段 |
| douban.go | 207-254 | 在switch case中提取评分 |
| main.go | 530-599 | 移除 `CheckRedirect`，优化日志 |
| main.go | 302-309 | 优先使用API评分，HTML抓取作为备用 |

---

## 测试命令

```bash
# 编译并运行
go run main.go

# 测试断连错误过滤
# 在浏览器中快速刷新页面

# 测试评分抓取
# 在表单中输入：https://movie.douban.com/subject/22735748/
# 点击添加，观察日志输出
```
