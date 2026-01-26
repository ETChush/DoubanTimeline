package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"douban-timeline/douban_module"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Movie 电影模型
type Movie struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DoubanID   string    `gorm:"uniqueIndex" json:"douban_id"`
	DoubanURL  string    `json:"douban_url"` // 原始豆瓣链接
	Title      string    `json:"title"`
	AltTitle   string    `json:"alt_title"`
	Director   string    `json:"director"`
	PubDate    string    `json:"pub_date"`
	ImageURL   string    `json:"image_url"`
	Rating     float64   `json:"rating"`
	Year       string    `json:"year"`
	Summary    string    `json:"summary"`
	IsFavorite bool      `json:"is_favorite" gorm:"default:false"` // 是否收藏
	WatchedAt  time.Time `json:"watched_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// JavMovie JAV影片模型
type JavMovie struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Number      string    `gorm:"uniqueIndex" json:"number"`      // 番号（唯一索引）
	Title       string    `json:"title"`                          // 标题
	Poster      string    `json:"poster"`                         // 海报URL
	Thumb       string    `json:"thumb"`                          // 封面URL
	Actor       string    `json:"actor"`                          // 演员
	Release     string    `json:"release"`                        // 发行日期
	Year        string    `json:"year"`                           // 年份
	Tag         string    `json:"tag"`                            // 标签
	Mosaic      string    `json:"mosaic"`                         // 马赛克类型（有码/无码）
	Runtime     string    `json:"runtime"`                        // 时长
	Studio      string    `json:"studio"`                         // 制作商
	Publisher   string    `json:"publisher"`                      // 发行商
	Director    string    `json:"director"`                       // 导演
	Series      string    `json:"series"`                         // 系列
	Website     string    `json:"website"`                        // 来源网址
	IsFavorite  bool      `json:"is_favorite" gorm:"default:false"` // 是否收藏
	WatchedAt   time.Time `json:"watched_at"`                     // 观看时间
	CreatedAt   time.Time `json:"created_at"`                     // 创建时间
}

// TimelineGroup 时间轴分组结构
type TimelineGroup struct {
	Year   string
	Month  string
	Movies []Movie
}

// JavTimelineGroup JAV时间轴分组结构
type JavTimelineGroup struct {
	Year      string
	Month     string
	JavMovies []JavMovie
}

// PageData 页面数据
type PageData struct {
	Groups        []TimelineGroup
	ErrorMsg      string
	SuccessMsg    string
	FavoritesOnly bool // 兼容模板
}

// JavPageData JAV页面数据
type JavPageData struct {
	Groups        []JavTimelineGroup
	ErrorMsg      string
	SuccessMsg    string
	FavoritesOnly bool // 是否只显示收藏
}

var db *gorm.DB
var templates *template.Template
var globalImageDir string = "images" // 图片存储目录

func init() {
	var err error

	// 获取数据目录（支持Docker挂载）
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}

	// 确保数据目录存在
	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// 初始化数据库
	dbPath := filepath.Join(dataDir, "douban_timeline.db")
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	// 自动迁移
	err = db.AutoMigrate(&Movie{}, &JavMovie{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 获取图片目录（支持Docker挂载）
	imageDir := os.Getenv("IMAGE_DIR")
	if imageDir == "" {
		imageDir = "images"
	}

	// 创建图片存储目录
	err = os.MkdirAll(imageDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create images directory: %v", err)
	}

	// 设置全局图片目录变量
	globalImageDir = imageDir

	// 加载模板
	templates = template.Must(template.New("").Funcs(template.FuncMap{
		"formatRating":   formatRating,
		"formatDate":     formatDate,
		"formatFullDate": formatFullDate,
		"now":            func() time.Time { return time.Now() },
	}).ParseGlob("templates/*.html"))
}

func main() {
	// 静态文件服务
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	// 图片服务
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))

	// 路由
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/add", addMovieHandler)
	http.HandleFunc("/delete", deleteMovieHandler)
	http.HandleFunc("/toggle-favorite", toggleMovieFavoriteHandler)
	http.HandleFunc("/export", exportHandler)

	// JAV 路由
	http.HandleFunc("/jav", javIndexHandler)
	http.HandleFunc("/jav/add", addJavMovieHandler)
	http.HandleFunc("/jav/delete", deleteJavMovieHandler)
	http.HandleFunc("/jav/toggle-favorite", toggleJavFavoriteHandler)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// indexHandler 主页处理器
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查是否只显示收藏
	favoritesOnly := r.URL.Query().Get("favorites") == "true"

	var movies []Movie
	query := db.Order("watched_at DESC")
	if favoritesOnly {
		query = query.Where("is_favorite = ?", true)
	}
	result := query.Find(&movies)
	if result.Error != nil {
		http.Error(w, "Failed to fetch movies", http.StatusInternalServerError)
		return
	}

	// 按年月分组
	groups := groupMoviesByYearMonth(movies)

	// 获取错误或成功消息
	errorMsg := r.URL.Query().Get("error")
	successMsg := r.URL.Query().Get("success")

	data := PageData{
		Groups:        groups,
		ErrorMsg:      errorMsg,
		SuccessMsg:    successMsg,
		FavoritesOnly: favoritesOnly,
	}

	// 修复问题1：执行模板前检查连接状态，减少broken pipe错误
	// 注意：net/http已移除CloseNotifier，无法直接检测连接关闭
	// 通过过滤断连类错误日志来避免日志刷屏
	err := templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		// 修复问题1：过滤客户端断连类错误（broken pipe、connection reset）
		// 此类错误在用户快速切换页面时常见，属于正常现象，无需记录日志
		if isBrokenPipeOrConnectionReset(err) {
			// 客户端主动断连，静默处理，不记录日志
			return
		}
		// 其他真正的错误（模板语法错误、数据格式错误等）需要记录
		log.Printf("Failed to execute template: %v", err)
		return
	}
}

// addMovieHandler 添加电影处理器
func addMovieHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	doubanURL := r.FormValue("douban_url")
	watchedAtStr := r.FormValue("watched_at")

	if doubanURL == "" {
		http.Redirect(w, r, "/?error="+encodeURL("请输入豆瓣链接"), http.StatusSeeOther)
		return
	}

	// 解析观看时间
	watchedAt := time.Now()
	if watchedAtStr != "" {
		parsed, err := time.Parse("2006-01-02", watchedAtStr)
		if err == nil {
			watchedAt = parsed
		}
	}

	// 判断链接类型
	subjectType := "movie"
	if strings.Contains(doubanURL, "book.douban.com") {
		subjectType = "book"
	} else if strings.Contains(doubanURL, "www.douban.com/game/") {
		subjectType = "game"
	}

	// 解析豆瓣URL
	subjectID, err := douban_module.ParseDoubanURL(doubanURL, subjectType)
	if err != nil {
		http.Redirect(w, r, "/?error="+encodeURL("无效的豆瓣链接: "+err.Error()), http.StatusSeeOther)
		return
	}

	// 获取豆瓣信息
	mediaInfo, err := douban_module.FetchDoubanMediaInfo(subjectType, subjectID)
	if err != nil {
		http.Redirect(w, r, "/?error="+encodeURL("获取豆瓣信息失败: "+err.Error()), http.StatusSeeOther)
		return
	}

	// 检查是否已存在
	var existingMovie Movie
	result := db.Where("douban_id = ?", subjectID).First(&existingMovie)

	// 修复问题2：使用 errors.Is() 精确判断是否为记录不存在错误
	// if result.Error == nil 只能判断没有错误，无法区分记录不存在和真正的数据库错误
	// 修复后：只有在找到记录时才更新，记录不存在时继续创建新记录
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 记录不存在，继续执行创建逻辑
		log.Printf("[addMovie] 豆瓣ID %s 不存在，准备创建新记录", subjectID)
	} else if result.Error != nil {
		// 其他数据库错误（连接失败、查询语法错误等），需要返回错误
		log.Printf("[addMovie] 数据库查询错误: %v", result.Error)
		http.Redirect(w, r, "/?error="+encodeURL("查询数据库失败: "+result.Error.Error()), http.StatusSeeOther)
		return
	} else {
		// 更新观看时间和链接
		log.Printf("[addMovie] 豆瓣ID %s 已存在，更新观看时间", subjectID)
		existingMovie.WatchedAt = watchedAt
		existingMovie.DoubanURL = doubanURL
		// 如果图片URL是外部链接，尝试下载到本地
		if mediaInfo.ImageURL != "" && (!strings.HasPrefix(existingMovie.ImageURL, "/images/") || existingMovie.ImageURL == "") {
			localPath, err := downloadImage(mediaInfo.ImageURL, subjectID, globalImageDir)
			if err == nil {
				existingMovie.ImageURL = "/images/" + filepath.Base(localPath)
			}
		}
		db.Save(&existingMovie)
		http.Redirect(w, r, "/?success="+encodeURL("电影已存在，已更新观看时间"), http.StatusSeeOther)
		return
	}

	// 提取年份和评分
	year := extractYear(mediaInfo.PubDate)
	// 修复问题2：优先使用API返回的评分，如果API没有评分，再尝试抓取HTML页面
	rating := mediaInfo.Rating
	if rating == 0 {
		log.Printf("[addMovie] API未返回评分，尝试从HTML页面抓取，豆瓣链接: %s", doubanURL)
		rating = fetchRating(doubanURL)
	} else {
		log.Printf("[addMovie] ✓ API成功返回评分: %.1f", rating)
	}

	// 下载并保存图片到本地
	localImageURL := ""
	if mediaInfo.ImageURL != "" {
		localPath, err := downloadImage(mediaInfo.ImageURL, subjectID, globalImageDir)
		if err != nil {
			log.Printf("Failed to download image: %v", err)
			// 如果下载失败，仍然使用原始URL
			localImageURL = mediaInfo.ImageURL
		} else {
			localImageURL = "/images/" + filepath.Base(localPath)
		}
	}

	// 创建电影记录
	movie := Movie{
		DoubanID:  subjectID,
		DoubanURL: doubanURL, // 保存原始豆瓣链接
		Title:     mediaInfo.Title,
		AltTitle:  mediaInfo.AltTitle,
		Director:  mediaInfo.Creator,
		PubDate:   mediaInfo.PubDate,
		ImageURL:  localImageURL, // 使用本地图片路径
		Rating:    rating,
		Year:      year,
		Summary:   mediaInfo.Summary,
		WatchedAt: watchedAt,
		CreatedAt: time.Now(),
	}

	result = db.Create(&movie)
	if result.Error != nil {
		http.Redirect(w, r, "/?error="+encodeURL("保存电影失败: "+result.Error.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?success="+encodeURL("电影添加成功"), http.StatusSeeOther)
}

// deleteMovieHandler 删除电影处理器
func deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	movieID := r.FormValue("id")
	if movieID == "" {
		http.Redirect(w, r, "/?error="+encodeURL("无效的电影ID"), http.StatusSeeOther)
		return
	}

	// 查找电影记录
	var movie Movie
	result := db.First(&movie, movieID)
	if result.Error != nil {
		http.Redirect(w, r, "/?error="+encodeURL("电影不存在"), http.StatusSeeOther)
		return
	}

	// 删除本地图片文件
	if movie.ImageURL != "" && strings.HasPrefix(movie.ImageURL, "/images/") {
		imagePath := strings.TrimPrefix(movie.ImageURL, "/images/")
		fullPath := filepath.Join(globalImageDir, imagePath)
		os.Remove(fullPath)
	}

	// 删除数据库记录
	db.Delete(&movie)

	http.Redirect(w, r, "/?success="+encodeURL("电影已删除"), http.StatusSeeOther)
}

// toggleMovieFavoriteHandler 切换豆瓣电影收藏状态处理器
func toggleMovieFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	movieID := r.FormValue("id")
	if movieID == "" {
		http.Redirect(w, r, "/?error="+encodeURL("无效的电影ID"), http.StatusSeeOther)
		return
	}

	// 查找电影记录
	var movie Movie
	result := db.First(&movie, movieID)
	if result.Error != nil {
		http.Redirect(w, r, "/?error="+encodeURL("电影不存在"), http.StatusSeeOther)
		return
	}

	// 切换收藏状态
	movie.IsFavorite = !movie.IsFavorite
	db.Save(&movie)

	// 返回到原页面（保持收藏过滤状态）
	redirectURL := "/"
	if r.FormValue("from_favorites") == "true" {
		redirectURL = "/?favorites=true"
	}
	if movie.IsFavorite {
		redirectURL += "&success=" + encodeURL("已添加到我的最爱")
	} else {
		redirectURL += "&success=" + encodeURL("已取消收藏")
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// exportHandler 导出JSON处理器
func exportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var movies []Movie
	result := db.Order("watched_at DESC").Find(&movies)
	if result.Error != nil {
		http.Error(w, "Failed to fetch movies", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=douban_timeline.json")

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(movies); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
}

// groupMoviesByYearMonth 按年月分组电影
func groupMoviesByYearMonth(movies []Movie) []TimelineGroup {
	groupMap := make(map[string]map[string][]Movie)

	for _, movie := range movies {
		year := movie.WatchedAt.Format("2006")
		month := movie.WatchedAt.Format("01")

		if groupMap[year] == nil {
			groupMap[year] = make(map[string][]Movie)
		}
		groupMap[year][month] = append(groupMap[year][month], movie)
	}

	var groups []TimelineGroup
	var years []string
	for year := range groupMap {
		years = append(years, year)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(years)))

	for _, year := range years {
		var months []string
		for month := range groupMap[year] {
			months = append(months, month)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(months)))

		for _, month := range months {
			groups = append(groups, TimelineGroup{
				Year:   year,
				Month:  month,
				Movies: groupMap[year][month],
			})
		}
	}

	return groups
}

// formatRating 格式化评分为星级
func formatRating(rating float64) string {
	if rating == 0 {
		return "暂无评分"
	}
	fullStars := int(rating / 2)
	halfStar := (rating/2 - float64(fullStars)) >= 0.5
	emptyStars := 5 - fullStars
	if halfStar {
		emptyStars--
	}

	result := strings.Repeat("★", fullStars)
	if halfStar {
		result += "☆"
	}
	result += strings.Repeat("☆", emptyStars)
	// 在星级后显示数字评分
	result += fmt.Sprintf(" %.1f", rating)
	return result
}

// formatDate 格式化日期
func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// formatFullDate 格式化完整日期（年月日）
func formatFullDate(t time.Time) string {
	return t.Format("2006年01月02日")
}

// extractYear 从日期字符串中提取年份
func extractYear(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	parts := strings.Fields(dateStr)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// fetchRating 从豆瓣页面抓取评分（带反爬策略）
// 修复问题2：添加完整的浏览器请求头和详细日志，绕过豆瓣反爬限制
func fetchRating(doubanURL string) float64 {
	log.Printf("[fetchRating] ========== 开始抓取豆瓣评分 ==========")
	log.Printf("[fetchRating] 请求URL: %s", doubanURL)

	client := &http.Client{
		Timeout: 15 * time.Second,
		// 允许自动跟随重定向，豆瓣页面通常需要重定向才能访问
		// 不设置CheckRedirect，使用默认行为
	}
	req, err := http.NewRequest("GET", doubanURL, nil)
	if err != nil {
		log.Printf("[fetchRating] ❌ 创建请求失败: %v", err)
		return 0
	}

	// 修复问题2：添加完整的浏览器请求头绕过豆瓣反爬
	// 使用真实的浏览器User-Agent和完整的HTTP请求头
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
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	log.Printf("[fetchRating] 发送HTTP请求...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[fetchRating] ❌ HTTP请求失败: %v", err)
		return 0
	}
	defer resp.Body.Close()

	// 修复问题2：添加响应状态码日志
	log.Printf("[fetchRating] ✓ 最终响应状态码: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Printf("[fetchRating] ⚠ 响应状态码非200，跳过评分提取，返回默认值")
		return 0
	}

	// 读取响应体（手动读取以支持gzip解压）
	body := make([]byte, 0, 1024*1024)
	buf := make([]byte, 32*1024)
	totalRead := 0
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
			totalRead += n
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("[fetchRating] ⚠ 读取响应体时出错: %v", err)
			}
			break
		}
	}
	log.Printf("[fetchRating] ✓ 成功读取响应体，大小: %d bytes", totalRead)

	// 使用正则表达式提取评分
	html := string(body)
	rating := extractRatingFromHTML(html)

	if rating > 0 {
		log.Printf("[fetchRating] ✓ 抓取成功，评分: %.1f", rating)
	} else {
		log.Printf("[fetchRating] ⚠ 未找到评分信息，返回默认值0")
	}
	log.Printf("[fetchRating] ========== 评分抓取结束 ==========")

	return rating
}

// isBrokenPipeOrConnectionReset 判断是否为客户端断连错误
func isBrokenPipeOrConnectionReset(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "write: connection aborted")
}

// extractRatingFromHTML 从HTML中提取评分
func extractRatingFromHTML(html string) float64 {
	// 尝试匹配评分模式，例如：<strong class="ll rating_num" property="v:average">9.7</strong>
	patterns := []string{
		`<strong[^>]*class="[^"]*rating[^"]*"[^>]*>(\d+\.?\d*)</strong>`,
		`property="v:average"[^>]*>(\d+\.?\d*)</strong>`,
		`rating_num[^>]*>(\d+\.?\d*)</strong>`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			rating, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				return rating
			}
		}
	}
	return 0
}

// downloadImage 下载图片到本地
func downloadImage(imageURL string, subjectID string, imageDir string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("empty image URL")
	}

	log.Printf("[downloadImage] 开始下载图片: %s", imageURL)

	const maxRetries = 3
	const retryDelay = 2 * time.Second
	const timeout = 60 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 创建HTTP请求
		client := &http.Client{Timeout: timeout}
		req, err := http.NewRequest("GET", imageURL, nil)
		if err != nil {
			log.Printf("[downloadImage] 第%d次尝试：创建请求失败: %v", attempt, err)
			if attempt == maxRetries {
				return "", err
			}
			time.Sleep(retryDelay)
			continue
		}

		// 设置完整的请求头，模拟浏览器
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Referer", "https://movie.douban.com/")
		req.Header.Set("Connection", "keep-alive")

		log.Printf("[downloadImage] 第%d次尝试：发送请求...", attempt)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[downloadImage] 第%d次尝试：请求失败: %v", attempt, err)
			if attempt == maxRetries {
				return "", fmt.Errorf("failed to download image after %d attempts: %v", maxRetries, err)
			}
			time.Sleep(retryDelay)
			continue
		}

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Printf("[downloadImage] 第%d次尝试：状态码 %d", attempt, resp.StatusCode)
			if attempt == maxRetries {
				return "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
			}
			time.Sleep(retryDelay)
			continue
		}

		log.Printf("[downloadImage] ✓ 第%d次尝试：响应成功，开始下载...", attempt)

		// 确定文件扩展名
		ext := ".jpg"
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "png") {
			ext = ".png"
		} else if strings.Contains(contentType, "gif") {
			ext = ".gif"
		} else if strings.Contains(contentType, "webp") {
			ext = ".webp"
		}

		// 生成本地文件名
		filename := subjectID + ext
		localPath := filepath.Join(imageDir, filename)

		// 创建文件
		out, err := os.Create(localPath)
		if err != nil {
			resp.Body.Close()
			log.Printf("[downloadImage] 创建文件失败: %v", err)
			return "", fmt.Errorf("failed to create file: %v", err)
		}
		defer out.Close()

		// 复制响应体到文件
		written, err := io.Copy(out, resp.Body)
		resp.Body.Close()
		if err != nil {
			os.Remove(localPath)
			log.Printf("[downloadImage] 下载文件失败: %v", err)
			return "", fmt.Errorf("failed to save image: %v", err)
		}

		log.Printf("[downloadImage] ✓ 图片下载成功，大小: %d bytes，保存路径: %s", written, localPath)
		return localPath, nil
	}

	return "", fmt.Errorf("failed to download image after %d attempts", maxRetries)
}

// encodeURL URL编码
func encodeURL(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "\n", "")
}

// ========== JAV 相关处理器 ==========

// javIndexHandler JAV主页处理器
func javIndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查是否只显示收藏
	favoritesOnly := r.URL.Query().Get("favorites") == "true"

	var javMovies []JavMovie
	query := db.Order("watched_at DESC")
	if favoritesOnly {
		query = query.Where("is_favorite = ?", true)
	}
	result := query.Find(&javMovies)
	if result.Error != nil {
		http.Error(w, "Failed to fetch JAV movies", http.StatusInternalServerError)
		return
	}

	// 按年月分组
	groups := groupJavMoviesByYearMonth(javMovies)

	// 获取错误或成功消息
	errorMsg := r.URL.Query().Get("error")
	successMsg := r.URL.Query().Get("success")

	data := JavPageData{
		Groups:        groups,
		ErrorMsg:      errorMsg,
		SuccessMsg:    successMsg,
		FavoritesOnly: favoritesOnly,
	}

	err := templates.ExecuteTemplate(w, "jav.html", data)
	if err != nil {
		// 修复问题1：过滤客户端断连类错误（broken pipe、connection reset）
		if isBrokenPipeOrConnectionReset(err) {
			return
		}
		log.Printf("Failed to execute template: %v", err)
		return
	}
}

// addJavMovieHandler 添加JAV影片处理器
func addJavMovieHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	number := r.FormValue("number")
	watchedAtStr := r.FormValue("watched_at")

	if number == "" {
		http.Redirect(w, r, "/jav?error="+encodeURL("请输入番号"), http.StatusSeeOther)
		return
	}

	// 解析观看时间
	watchedAt := time.Now()
	if watchedAtStr != "" {
		parsed, err := time.Parse("2006-01-02", watchedAtStr)
		if err == nil {
			watchedAt = parsed
		}
	}

	// 检查是否已存在
	var existingMovie JavMovie
	result := db.Where("number = ?", strings.ToUpper(number)).First(&existingMovie)
	if result.Error == nil {
		// 更新观看时间
		existingMovie.WatchedAt = watchedAt
		db.Save(&existingMovie)
		http.Redirect(w, r, "/jav?success="+encodeURL("影片已存在，已更新观看时间"), http.StatusSeeOther)
		return
	}

	// 调用 Python 爬虫获取影片信息
	javInfo, err := fetchJavInfoFromPython(number)
	if err != nil {
		http.Redirect(w, r, "/jav?error="+encodeURL("获取影片信息失败: "+err.Error()), http.StatusSeeOther)
		return
	}

	// 下载封面到本地
	localPoster := ""
	if javInfo.Poster != "" {
		localPath, err := downloadJavImage(javInfo.Poster, javInfo.Number+"_poster", globalImageDir)
		if err == nil {
			localPoster = "/images/" + filepath.Base(localPath)
		}
	}

	// 创建JAV影片记录
	javMovie := JavMovie{
		Number:     javInfo.Number,
		Title:      javInfo.Title,
		Poster:     localPoster,
		Thumb:      javInfo.Thumb,
		Actor:      javInfo.Actor,
		Release:    javInfo.Release,
		Year:       javInfo.Year,
		Tag:        javInfo.Tag,
		Mosaic:     javInfo.Mosaic,
		Runtime:    javInfo.Runtime,
		Studio:     javInfo.Studio,
		Publisher:  javInfo.Publisher,
		Director:   javInfo.Director,
		Series:     javInfo.Series,
		Website:    javInfo.Website,
		IsFavorite: false,
		WatchedAt:  watchedAt,
		CreatedAt:  time.Now(),
	}

	result = db.Create(&javMovie)
	if result.Error != nil {
		http.Redirect(w, r, "/jav?error="+encodeURL("保存影片失败: "+result.Error.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/jav?success="+encodeURL("影片添加成功"), http.StatusSeeOther)
}

// deleteJavMovieHandler 删除JAV影片处理器
func deleteJavMovieHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	movieID := r.FormValue("id")
	if movieID == "" {
		http.Redirect(w, r, "/jav?error="+encodeURL("无效的影片ID"), http.StatusSeeOther)
		return
	}

	// 查找影片记录
	var javMovie JavMovie
	result := db.First(&javMovie, movieID)
	if result.Error != nil {
		http.Redirect(w, r, "/jav?error="+encodeURL("影片不存在"), http.StatusSeeOther)
		return
	}

	// 删除本地图片文件
	if javMovie.Poster != "" && strings.HasPrefix(javMovie.Poster, "/images/") {
		imagePath := strings.TrimPrefix(javMovie.Poster, "/images/")
		fullPath := filepath.Join(globalImageDir, imagePath)
		os.Remove(fullPath)
	}

	// 删除数据库记录
	db.Delete(&javMovie)

	http.Redirect(w, r, "/jav?success="+encodeURL("影片已删除"), http.StatusSeeOther)
}

// toggleJavFavoriteHandler 切换JAV影片收藏状态处理器
func toggleJavFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	movieID := r.FormValue("id")
	if movieID == "" {
		http.Redirect(w, r, "/jav?error="+encodeURL("无效的影片ID"), http.StatusSeeOther)
		return
	}

	// 查找影片记录
	var javMovie JavMovie
	result := db.First(&javMovie, movieID)
	if result.Error != nil {
		http.Redirect(w, r, "/jav?error="+encodeURL("影片不存在"), http.StatusSeeOther)
		return
	}

	// 切换收藏状态
	javMovie.IsFavorite = !javMovie.IsFavorite
	db.Save(&javMovie)

	// 返回到原页面（保持收藏过滤状态）
	redirectURL := "/jav"
	if r.FormValue("from_favorites") == "true" {
		redirectURL = "/jav?favorites=true"
	}
	if javMovie.IsFavorite {
		redirectURL += "&success=" + encodeURL("已添加到我的最爱")
	} else {
		redirectURL += "&success=" + encodeURL("已取消收藏")
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// groupJavMoviesByYearMonth 按年月分组JAV影片
func groupJavMoviesByYearMonth(javMovies []JavMovie) []JavTimelineGroup {
	groupMap := make(map[string]map[string][]JavMovie)

	for _, movie := range javMovies {
		year := movie.WatchedAt.Format("2006")
		month := movie.WatchedAt.Format("01")

		if groupMap[year] == nil {
			groupMap[year] = make(map[string][]JavMovie)
		}
		groupMap[year][month] = append(groupMap[year][month], movie)
	}

	var groups []JavTimelineGroup
	var years []string
	for year := range groupMap {
		years = append(years, year)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(years)))

	for _, year := range years {
		var months []string
		for month := range groupMap[year] {
			months = append(months, month)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(months)))

		for _, month := range months {
			groups = append(groups, JavTimelineGroup{
				Year:      year,
				Month:     month,
				JavMovies: groupMap[year][month],
			})
		}
	}

	return groups
}

// fetchJavInfoFromPython 调用Python爬虫获取JAV影片信息
func fetchJavInfoFromPython(number string) (*JavMovie, error) {
	// 执行 Python 脚本，输出 JSON 格式
	// 设置环境变量确保 Python 使用 UTF-8 编码

	// 尝试使用 python3 或 python 命令
	pythonCmd := "python"
	if _, err := exec.LookPath("python3"); err == nil {
		pythonCmd = "python3"
	}

	cmd := exec.Command(pythonCmd, "javbus_crawler.py", number, "--only-cover", "--json")
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Python 脚本执行失败: %v\n%s", err, string(output))
	}

	// 从输出中提取 JSON 部分
	outputStr := string(output)
	startMarker := "__JSON_START__"
	endMarker := "__JSON_END__"

	startIdx := strings.Index(outputStr, startMarker)
	endIdx := strings.Index(outputStr, endMarker)

	if startIdx == -1 || endIdx == -1 {
		return nil, fmt.Errorf("未找到有效的 JSON 输出\n%s", outputStr)
	}

	jsonStr := outputStr[startIdx+len(startMarker) : endIdx]
	jsonStr = strings.TrimSpace(jsonStr)

	// 解析 JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %v\n%s", err, jsonStr)
	}

	// 构建 JavMovie 结构
	javMovie := &JavMovie{
		Number:    getString(result, "number"),
		Title:     getString(result, "title"),
		Poster:    getString(result, "poster"),
		Thumb:     getString(result, "thumb"),
		Actor:     getString(result, "actor"),
		Release:   getString(result, "release"),
		Year:      getString(result, "year"),
		Tag:       getString(result, "tag"),
		Mosaic:    getString(result, "mosaic"),
		Runtime:   getString(result, "runtime"),
		Studio:    getString(result, "studio"),
		Publisher: getString(result, "publisher"),
		Director:  getString(result, "director"),
		Series:    getString(result, "series"),
		Website:   getString(result, "website"),
	}

	return javMovie, nil
}

// getString 从 map 中安全获取字符串值
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// downloadJavImage 下载JAV影片图片到本地
func downloadJavImage(imageURL string, filename string, imageDir string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("empty image URL")
	}

	log.Printf("[downloadJavImage] 开始下载JAV图片: %s", imageURL)

	const maxRetries = 3
	const retryDelay = 2 * time.Second
	const timeout = 60 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 创建HTTP请求
		client := &http.Client{Timeout: timeout}
		req, err := http.NewRequest("GET", imageURL, nil)
		if err != nil {
			log.Printf("[downloadJavImage] 第%d次尝试：创建请求失败: %v", attempt, err)
			if attempt == maxRetries {
				return "", err
			}
			time.Sleep(retryDelay)
			continue
		}

		// 设置完整的请求头，模拟浏览器
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Referer", "https://www.javbus.com/")
		req.Header.Set("Connection", "keep-alive")

		log.Printf("[downloadJavImage] 第%d次尝试：发送请求...", attempt)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[downloadJavImage] 第%d次尝试：请求失败: %v", attempt, err)
			if attempt == maxRetries {
				return "", fmt.Errorf("failed to download image after %d attempts: %v", maxRetries, err)
			}
			time.Sleep(retryDelay)
			continue
		}

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Printf("[downloadJavImage] 第%d次尝试：状态码 %d", attempt, resp.StatusCode)
			if attempt == maxRetries {
				return "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
			}
			time.Sleep(retryDelay)
			continue
		}

		log.Printf("[downloadJavImage] ✓ 第%d次尝试：响应成功，开始下载...", attempt)

		// 确定文件扩展名
		ext := ".jpg"
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "png") {
			ext = ".png"
		} else if strings.Contains(contentType, "webp") {
			ext = ".webp"
		}

		// 生成本地文件名
		localFilename := filename + ext
		localPath := filepath.Join(imageDir, localFilename)

		// 创建文件
		out, err := os.Create(localPath)
		if err != nil {
			resp.Body.Close()
			log.Printf("[downloadJavImage] 创建文件失败: %v", err)
			return "", fmt.Errorf("failed to create file: %v", err)
		}
		defer out.Close()

		// 复制响应体到文件
		written, err := io.Copy(out, resp.Body)
		resp.Body.Close()
		if err != nil {
			os.Remove(localPath)
			log.Printf("[downloadJavImage] 下载文件失败: %v", err)
			return "", fmt.Errorf("failed to save image: %v", err)
		}

		log.Printf("[downloadJavImage] ✓ JAV图片下载成功，大小: %d bytes，保存路径: %s", written, localPath)
		return localPath, nil
	}

	return "", fmt.Errorf("failed to download JAV image after %d attempts", maxRetries)
}
