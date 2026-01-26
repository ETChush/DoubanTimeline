# 问题修复总结

## 修复的两个核心问题

### 问题1：高频 `broken pipe`/`connection reset` 日志警告

**现象：**
```
Failed to execute template: write tcp [::1]:8080->[::1]:58692: write: broken pipe
Failed to execute template: write tcp [::1]:8080->[::1]:58726: write: connection reset by peer
```

**原因：**
客户端（浏览器）在服务器渲染模板前关闭连接，导致 `templates.ExecuteTemplate` 写入响应失败，日志刷屏。

**修复方案：**
1. 在 `indexHandler` 函数中添加断连错误过滤（已完成）
2. 在 `javIndexHandler` 函数中添加断连错误过滤（本次新增）

**修改位置：**
- `main.go:214-225` - indexHandler 错误过滤
- `main.go:750-758` - javIndexHandler 错误过滤（本次新增）

**关键代码：**
```go
err := templates.ExecuteTemplate(w, "index.html", data)
if err != nil {
    // 过滤客户端断连类错误（broken pipe、connection reset）
    if isBrokenPipeOrConnectionReset(err) {
        return  // 静默处理，不记录日志
    }
    log.Printf("Failed to execute template: %v", err)
    return
}
```

---

### 问题2：豆瓣评分抓取失败

**现象：**
```
/app/main.go:268 record not found
[0.593ms] [rows:0] SELECT * FROM `movies` WHERE douban_id = "22735748" ORDER BY `movies`.`id` LIMIT 1
```
- 数据库查询无记录（正常逻辑），但未执行后续的 `fetchRating` 函数抓取评分
- 日志中完全没有 `fetchRating` 的执行痕迹
- 豆瓣对普通 `http.Get` 请求有反爬限制，直接请求会返回403或302

**修复方案：**

#### 方案A：优先使用豆瓣API获取评分（推荐）
- 修改豆瓣API数据结构，添加评分字段
- 从API响应中提取评分
- 避免反爬问题，数据更可靠

**修改位置：**
- `douban_module/douban.go:29-33` - 添加 `DoubanRating` 结构体
- `douban_module/douban.go:45-54` - 在各Subject中添加 `Rating` 字段
- `douban_module/douban.go:68-77` - 在 `MediaSubject` 中添加 `Rating` 字段
- `douban_module/douban.go:207-254` - 在switch case中提取评分
- `main.go:302-309` - 优先使用API评分，HTML抓取作为备用

**关键代码：**
```go
// 优先使用API返回的评分，如果API没有评分，再尝试抓取HTML页面
rating := mediaInfo.Rating
if rating == 0 {
    log.Printf("[addMovie] API未返回评分，尝试从HTML页面抓取，豆瓣链接: %s", doubanURL)
    rating = fetchRating(doubanURL)
} else {
    log.Printf("[addMovie] ✓ API成功返回评分: %.1f", rating)
}
```

#### 方案B：改进HTML抓取的反爬策略（备用）
- 移除 `CheckRedirect` 限制，允许跟随302重定向
- 添加完整的浏览器请求头（User-Agent、Accept、Sec-Fetch-*等）
- 优化日志输出，便于追踪执行流程

**修改位置：**
- `main.go:530-599` - fetchRating 函数改进

**关键代码：**
```go
client := &http.Client{
    Timeout: 15 * time.Second,
    // 允许自动跟随重定向（移除了CheckRedirect）
}
```

**完整的请求头：**
```go
headers := map[string]string{
    "User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Accept":         "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
    "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
    "Accept-Encoding": "gzip, deflate, br",
    "Referer":         "https://www.douban.com/",
    "Connection":      "keep-alive",
    "Upgrade-Insecure-Requests": "1",
    "Cache-Control":   "max-age=0",
    "Sec-Fetch-Dest":  "document",
    "Sec-Fetch-Mode":  "navigate",
    "Sec-Fetch-Site":  "none",
    "Sec-Fetch-User":  "?1",
}
```

---

## 修改文件清单

### 文件1：main.go
| 修改点 | 行号 | 修改类型 | 说明 |
|--------|------|----------|------|
| 1 | 214-225 | 已有 | indexHandler 断连错误过滤 |
| 2 | 280-291 | 修改 | 添加调试日志 |
| 3 | 302-309 | 修改 | 优先使用API评分 |
| 4 | 530-599 | 修改 | fetchRating 移除CheckRedirect，优化日志 |
| 5 | 750-758 | 新增 | javIndexHandler 断连错误过滤 |

### 文件2：douban_module/douban.go
| 修改点 | 行号 | 修改类型 | 说明 |
|--------|------|----------|------|
| 1 | 29-33 | 新增 | DoubanRating 结构体 |
| 2 | 45-54 | 修改 | 添加 Rating 字段到各Subject |
| 3 | 68-77 | 修改 | 添加 Rating 字段到 MediaSubject |
| 4 | 197-254 | 修改 | 提取并返回评分 |

---

## 验证方法

### 验证问题1修复
1. 启动服务：`go run main.go`
2. 在浏览器中快速刷新页面或切换标签页
3. 观察日志输出
4. ✅ 应该看不到 "broken pipe" 或 "connection reset" 错误

### 验证问题2修复
1. 启动服务
2. 在表单中输入豆瓣链接：`https://movie.douban.com/subject/22735748/`
3. 点击添加
4. 观察日志输出
5. ✅ 应该看到评分被成功获取（优先API，备用HTML抓取）

---

## 技术说明

### 为什么使用 errors.Is() 而不是直接比较？
- `errors.Is()` 可以正确包装和解包错误
- 符合 Go 1.13+ 的错误处理最佳实践
- 能够处理错误包装链，确保精确匹配 `gorm.ErrRecordNotFound`

### 为什么添加 Sec-Fetch-* 头部？
- 现代浏览器会自动添加这些头部
- 豆瓣会检测这些头部来判断是否为真实浏览器
- 不添加这些头部可能导致 403 Forbidden 错误

### 为什么移除 CheckRedirect？
- 豆瓣页面需要重定向才能访问
- 禁用自动重定向导致返回 302 而不是跟随重定向
- 允许重定向后可以正常访问豆瓣页面

### 为什么优先使用API？
- API 返回的数据更可靠
- 不易被反爬机制拦截
- 减少HTML解析的复杂度
- 提高评分获取成功率

---

## 日志输出示例

### 成功添加新电影（API返回评分）
```
[addMovie] 豆瓣ID 22735748 不存在，准备创建新记录
[addMovie] ✓ API成功返回评分: 9.7
```

### 成功添加新电影（HTML抓取评分）
```
[addMovie] 豆瓣ID 22735748 不存在，准备创建新记录
[addMovie] API未返回评分，尝试从HTML页面抓取，豆瓣链接: https://movie.douban.com/subject/22735748/
[fetchRating] ========== 开始抓取豆瓣评分 ==========
[fetchRating] 请求URL: https://movie.douban.com/subject/22735748/
[fetchRating] 发送HTTP请求...
[fetchRating] ✓ 最终响应状态码: 200
[fetchRating] ✓ 成功读取响应体，大小: 123456 bytes
[fetchRating] ✓ 抓取成功，评分: 9.7
[fetchRating] ========== 评分抓取结束 ==========
```

### 更新已有电影
```
[addMovie] 豆瓣ID 22735748 已存在，更新观看时间
```

---

## 注意事项

1. **测试环境**：建议在测试环境中先验证修复效果
2. **日志监控**：关注日志输出，确认评分获取流程正常
3. **错误处理**：如果仍然出现评分获取失败，检查网络连接和豆瓣API状态
4. **性能考虑**：API响应更快，优先使用API可以提高性能

---

## 后续优化建议

1. **添加评分缓存**：避免重复抓取同一电影的评分
2. **异步抓取**：将评分抓取改为异步任务，不阻塞添加流程
3. **重试机制**：添加评分抓取失败的重试逻辑
4. **评分更新**：定期更新已存在电影的评分
5. **监控告警**：添加评分获取失败的监控和告警机制
