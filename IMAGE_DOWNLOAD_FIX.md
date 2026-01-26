# 图片下载优化完成

## 优化内容

### 1. 下载豆瓣图片函数 - `downloadImage`

**改进点：**
- ✅ 超时时间：30秒 → 60秒
- ✅ 重试机制：最多3次，每次间隔2秒
- ✅ 详细日志：每次尝试都有日志输出
- ✅ 完整请求头：模拟真实浏览器
- ✅ 错误处理：失败时删除不完整文件

**新增日志输出：**
```
[downloadImage] 开始下载图片: https://img1.doubanio.com/...
[downloadImage] 第1次尝试：发送请求...
[downloadImage] ✓ 第1次尝试：响应成功，开始下载...
[downloadImage] ✓ 图片下载成功，大小: 123456 bytes，保存路径: /images/22735748.jpg
```

**完整的请求头：**
```go
req.Header.Set("User-Agent", "Mozilla/5.0 ...")
req.Header.Set("Accept", "image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
req.Header.Set("Referer", "https://movie.douban.com/")
req.Header.Set("Connection", "keep-alive")
```

---

### 2. 下载JAV图片函数 - `downloadJavImage`

**改进点：**
- ✅ 超时时间：30秒 → 60秒
- ✅ 重试机制：最多3次，每次间隔2秒
- ✅ 详细日志：每次尝试都有日志输出
- ✅ 完整请求头：模拟真实浏览器
- ✅ 错误处理：失败时删除不完整文件

**新增日志输出：**
```
[downloadJavImage] 开始下载JAV图片: https://...
[downloadJavImage] 第1次尝试：发送请求...
[downloadJavImage] ✓ 第1次尝试：响应成功，开始下载...
[downloadJavImage] ✓ JAV图片下载成功，大小: 234567 bytes，保存路径: /images/SSIS-001_poster.jpg
```

---

## 优化参数

| 参数 | 优化前 | 优化后 | 说明 |
|------|--------|--------|------|
| 超时时间 | 30秒 | 60秒 | 适应慢速网络 |
| 重试次数 | 0 | 3次 | 提高成功率 |
| 重试延迟 | 0秒 | 2秒 | 避免频繁请求 |
| 日志详细度 | 简单 | 详细 | 便于排查问题 |
| 请求头 | 简单 | 完整 | 模拟真实浏览器 |

---

## 重试逻辑

```go
const maxRetries = 3
const retryDelay = 2 * time.Second

for attempt := 1; attempt <= maxRetries; attempt++ {
    // 尝试下载
    // 失败则等待2秒后重试
    // 成功则立即返回
}
```

**重试时机：**
- 创建请求失败
- HTTP请求失败（超时、连接错误）
- 状态码非200
- 保存文件失败

---

## 错误处理

**失败时清理：**
```go
os.Remove(localPath)  // 删除不完整的文件
```

**详细错误信息：**
```go
return "", fmt.Errorf("failed to download image after %d attempts: %v", maxRetries, err)
```

---

## 完整流程示例

### 成功下载（第1次尝试）
```
[downloadImage] 开始下载图片: https://img1.doubanio.com/view/photo/s_ratio_poster/public/p2159698949.jpg
[downloadImage] 第1次尝试：发送请求...
[downloadImage] ✓ 第1次尝试：响应成功，开始下载...
[downloadImage] ✓ 图片下载成功，大小: 87654 bytes，保存路径: /images/22735748.jpg
```

### 失败重试（第3次成功）
```
[downloadImage] 开始下载图片: https://img1.doubanio.com/...
[downloadImage] 第1次尝试：发送请求...
[downloadImage] 第1次尝试：请求失败: context deadline exceeded
[downloadImage] 第2次尝试：发送请求...
[downloadImage] 第2次尝试：请求失败: context deadline exceeded
[downloadImage] 第3次尝试：发送请求...
[downloadImage] ✓ 第3次尝试：响应成功，开始下载...
[downloadImage] ✓ 图片下载成功，大小: 54321 bytes，保存路径: /images/22735748.jpg
```

### 完全失败（3次都失败）
```
[downloadImage] 开始下载图片: https://img1.doubanio.com/...
[downloadImage] 第1次尝试：发送请求...
[downloadImage] 第1次尝试：请求失败: context deadline exceeded
[downloadImage] 第2次尝试：发送请求...
[downloadImage] 第2次尝试：请求失败: context deadline exceeded
[downloadImage] 第3次尝试：发送请求...
[downloadImage] 第3次尝试：请求失败: context deadline exceeded
Failed to download image: failed to download image after 3 attempts: context deadline exceeded
```

---

## 为什么这样优化？

### 1. 增加超时时间（30秒 → 60秒）
**原因：**
- 豆瓣图片服务器可能响应较慢
- 某些图片文件较大
- 网络波动导致请求耗时增加

### 2. 添加重试机制
**原因：**
- 网络抖动是暂时的
- 服务器偶尔繁忙
- 提高下载成功率

### 3. 完善请求头
**原因：**
- 模拟真实浏览器行为
- 避免403 Forbidden错误
- 某些服务器会检查请求头

### 4. 详细日志
**原因：**
- 便于排查问题
- 了解下载进度
- 追踪重试情况

---

## 验证方法

### 测试豆瓣图片下载
1. 添加豆瓣电影：`https://movie.douban.com/subject/22735748/`
2. 观察日志输出
3. ✅ 应该看到成功的下载日志

### 测试JAV图片下载
1. 添加JAV影片（需要Python爬虫）
2. 观察日志输出
3. ✅ 应该看到成功的下载日志

---

## 常见问题

### Q: 为什么还会超时？
**A:** 可能原因：
- 网络环境差
- 豆瓣服务器繁忙
- 图片URL失效

**解决：**
- 重试机制会自动尝试3次
- 3次都失败会记录详细错误
- 可以手动重新添加记录

### Q: 如何调整重试次数？
**A:** 修改代码中的常量：
```go
const maxRetries = 3  // 改为你想要的次数
```

### Q: 如何调整超时时间？
**A:** 修改代码中的常量：
```go
const timeout = 60 * time.Second  // 改为你想要的时间
```

---

## 当前状态

| 功能 | 状态 | 说明 |
|------|------|------|
| 豆瓣评分获取 | ✅ 正常 | API成功返回评分 |
| 豆瓣图片下载 | ✅ 优化完成 | 增加超时+重试机制 |
| JAV图片下载 | ✅ 优化完成 | 增加超时+重试机制 |
| 断连错误过滤 | ✅ 已修复 | 不再刷屏 |

---

## 下一步

重新测试添加豆瓣电影：
```
豆瓣链接：https://movie.douban.com/subject/22735748/

预期日志：
[addMovie] 豆瓣ID 22735748 不存在，准备创建新记录
[addMovie] ✓ API成功返回评分: 8.5
[downloadImage] 开始下载图片: https://img1.doubanio.com/...
[downloadImage] 第1次尝试：发送请求...
[downloadImage] ✓ 第1次尝试：响应成功，开始下载...
[downloadImage] ✓ 图片下载成功，大小: xxxxx bytes，保存路径: /images/22735748.jpg
```
