package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
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
	ID        uint      `gorm:"primaryKey" json:"id"`
	DoubanID  string    `gorm:"uniqueIndex" json:"douban_id"`
	DoubanURL string    `json:"douban_url"` // 原始豆瓣链接
	Title     string    `json:"title"`
	AltTitle  string    `json:"alt_title"`
	Director  string    `json:"director"`
	PubDate   string    `json:"pub_date"`
	ImageURL  string    `json:"image_url"`
	Rating    float64   `json:"rating"`
	Year      string    `json:"year"`
	Summary   string    `json:"summary"`
	WatchedAt time.Time `json:"watched_at"`
	CreatedAt time.Time `json:"created_at"`
}

// TimelineGroup 时间轴分组结构
type TimelineGroup struct {
	Year   string
	Month  string
	Movies []Movie
}

// PageData 页面数据
type PageData struct {
	Groups     []TimelineGroup
	ErrorMsg   string
	SuccessMsg string
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
	err = db.AutoMigrate(&Movie{})
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
	http.HandleFunc("/export", exportHandler)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// indexHandler 主页处理器
func indexHandler(w http.ResponseWriter, r *http.Request) {
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

	// 按年月分组
	groups := groupMoviesByYearMonth(movies)

	// 获取错误或成功消息
	errorMsg := r.URL.Query().Get("error")
	successMsg := r.URL.Query().Get("success")

	data := PageData{
		Groups:     groups,
		ErrorMsg:   errorMsg,
		SuccessMsg: successMsg,
	}

	err := templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	if result.Error == nil {
		// 更新观看时间和链接
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
	rating := fetchRating(doubanURL)

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

// fetchRating 从豆瓣页面抓取评分
func fetchRating(doubanURL string) float64 {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", doubanURL, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0
	}

	// 读取响应体
	body := make([]byte, 0, 1024*1024)
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	// 使用正则表达式提取评分
	html := string(body)
	rating := extractRatingFromHTML(html)
	return rating
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

	// 创建HTTP请求
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://movie.douban.com/")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

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
		return "", err
	}
	defer out.Close()

	// 复制响应体到文件
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return localPath, nil
}

// encodeURL URL编码
func encodeURL(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "\n", "")
}
