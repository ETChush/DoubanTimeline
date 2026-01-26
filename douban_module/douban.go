package douban_module

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// 豆瓣API配置
const (
	doubanAPIHost  = "frodo.douban.com"
	doubanAPIToken = "0ac44ae016490db2204ce0a042db2916"
)

// DoubanCover 豆瓣封面图片
 type DoubanCover struct {
	Normal string `json:"normal"`
}

// DoubanDirector 豆瓣导演信息
 type DoubanDirector struct {
	Name string `json:"name"`
}

// DoubanBookSubject 豆瓣图书主题
type DoubanBookSubject struct {
	Title        string         `json:"title"`
	AltTitle     string         `json:"book_subtitle"`
	PubDate      []string       `json:"pubdate"`
	Author       []string       `json:"author"`
	Press        []string       `json:"press"`
	CardSubtitle string         `json:"card_subtitle"`
	Intro        string         `json:"intro"`
	Type         string         `json:"type"`
	Cover        DoubanCover    `json:"pic"`
	Rating       *DoubanRating  `json:"rating"`
}

// DoubanMovieSubject 豆瓣电影主题
 type DoubanMovieSubject struct {
	Title        string           `json:"title"`
	AltTitle     string           `json:"original_title"`
	PubDate      []string         `json:"pubdate"`
	Directors    []DoubanDirector `json:"directors"`
	CardSubtitle string           `json:"card_subtitle"`
	Intro        string           `json:"intro"`
	Type         string           `json:"type"`
	Cover        DoubanCover      `json:"pic"`
	Rating       *DoubanRating    `json:"rating"`
}

// DoubanGameSubject 豆瓣游戏主题
 type DoubanGameSubject struct {
	Title       string      `json:"title"`
	TitleCN     string      `json:"cn_name"`
	ReleaseDate string      `json:"release_date"`
	Developer   []string    `json:"developers"`
	Publisher   []string    `json:"publishers"`
	Intro       string      `json:"intro"`
	Type        string      `json:"type"`
	Cover       DoubanCover `json:"pic"`
	Rating      *DoubanRating `json:"rating"`
}

// DoubanRating 豆瓣评分信息
type DoubanRating struct {
	Count int     `json:"count"`
	Value float64 `json:"value"`
	Max   int     `json:"max"`
}

// MediaSubject 统一的媒体主题结构
type MediaSubject struct {
	Title    string
	AltTitle string
	Creator  string
	Press    string
	PubDate  string
	Summary  string
	ImageURL string
	Rating   float64
}

// ParseDoubanURL 解析豆瓣URL，提取subject ID
func ParseDoubanURL(externalURL string, subjectType string) (string, error) {
	// 验证URL格式
	pattern := regexp.MustCompile(`^https://(?:(?:www|book|movie)\.douban\.com)/(?:game|subject)/(\d+)/?$`)
	matched := pattern.MatchString(externalURL)
	if !matched {
		return "", errors.New("invalid douban URL format")
	}

	// 提取subject ID
	matches := pattern.FindStringSubmatch(externalURL)
	if len(matches) <= 1 {
		return "", errors.New("failed to extract subject ID from URL")
	}
	subjectID := matches[1]

	// 验证主机名是否与主题类型匹配
	u, err := url.Parse(externalURL)
	if err != nil {
		return "", err
	}
	urlHost := strings.Split(u.Hostname(), ":")[0]

	var validHosts []string
	switch subjectType {
	case "book":
		validHosts = []string{"book.douban.com"}
	case "movie", "tv", "anime":
		validHosts = []string{"movie.douban.com"}
	case "game":
		validHosts = []string{"www.douban.com"}
	default:
		return "", errors.New("unknown subject type")
	}

	isValidHost := false
	for _, host := range validHosts {
		if urlHost == host {
			isValidHost = true
			break
		}
	}
	if !isValidHost {
		return "", fmt.Errorf("invalid host for subject type %s. supported hosts: %s", subjectType, strings.Join(validHosts, ", "))
	}

	return subjectID, nil
}

// FetchDoubanMediaInfo 从豆瓣获取媒体信息
func FetchDoubanMediaInfo(subjectType, subjectID string) (MediaSubject, error) {
	var apiSubjectType string
	if subjectType == "book" {
		apiSubjectType = "book"
	} else if subjectType == "game" {
		apiSubjectType = "game"
	} else {
		apiSubjectType = "movie"
	}

	// 构建API请求URL
	apiURL := fmt.Sprintf("https://%s/api/v2/%s/%s", doubanAPIHost, apiSubjectType, subjectID)
	params := fmt.Sprintf("?apiKey=%s", doubanAPIToken)
	requestURL := apiURL + params

	// 设置请求头
	headers := map[string]string{
		"Host":       doubanAPIHost,
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 15_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.16(0x18001023) NetType/WIFI Language/zh_CN",
		"Referer":    "https://servicewechat.com/wx2f9b06c1de1ccfca/84/page-frame.html",
	}

	// 发送API请求
	jsonData, err := requestAPI(requestURL, headers)
	if err != nil {
		return MediaSubject{}, err
	}

	// 解析JSON数据
	var doubanSubject interface{}
	switch subjectType {
	case "book":
		doubanSubject = &DoubanBookSubject{}
	case "game":
		doubanSubject = &DoubanGameSubject{}
	default:
		doubanSubject = &DoubanMovieSubject{}
	}

	if err := json.Unmarshal(jsonData, doubanSubject); err != nil {
		return MediaSubject{}, err
	}

	// 提取并转换数据
	var (
		title    string
		altTitle string
		creator  string
		press    string
		pubDate  string
		summary  string
		imageURL string
		rating   float64
	)

	switch subject := doubanSubject.(type) {
	case *DoubanBookSubject:
		title = subject.Title
		altTitle = subject.AltTitle
		creator = joinStringsWithSlash(subject.Author)
		press = joinStringsWithSlash(subject.Press)
		pubDate = getFirstPubdate(subject.PubDate)
		summary = subject.Intro
		imageURL = subject.Cover.Normal
		// 提取评分
		if subject.Rating != nil {
			rating = subject.Rating.Value
		}
	case *DoubanMovieSubject:
		title = subject.Title
		altTitle = subject.AltTitle
		creator = getCreator(subject.Directors)
		tagParts := strings.Split(subject.CardSubtitle, " / ")
		if len(tagParts) > 1 {
			press = tagParts[1]
		} else {
			press = ""
		}
		pubDate = getFirstPubdate(subject.PubDate)
		summary = subject.Intro
		imageURL = subject.Cover.Normal
		// 提取评分
		if subject.Rating != nil {
			rating = subject.Rating.Value
		}
	case *DoubanGameSubject:
		if strings.Contains(subject.Title, subject.TitleCN) {
			title = subject.TitleCN
			altTitle = strings.TrimSpace(strings.Replace(subject.Title, subject.TitleCN, "", 1))
		} else {
			title = subject.Title
			altTitle = subject.TitleCN
		}
		creator = joinStringsWithSlash(subject.Developer)
		press = joinStringsWithSlash(subject.Publisher)
		pubDate = subject.ReleaseDate
		summary = subject.Intro
		imageURL = subject.Cover.Normal
		// 提取评分
		if subject.Rating != nil {
			rating = subject.Rating.Value
		}
	}

	return MediaSubject{
		Title:    title,
		AltTitle: altTitle,
		Creator:  creator,
		Press:    press,
		PubDate:  pubDate,
		Summary:  summary,
		ImageURL: imageURL,
		Rating:   rating,
	}, nil
}

// requestAPI 发送API请求
func requestAPI(requestURL string, headers map[string]string) ([]byte, error) {
	const maxRetries = 2
	const retryDelay = 2 * time.Second
	client := &http.Client{Timeout: 10 * time.Second}
	var body []byte
	var err error

	for i := 0; i < maxRetries; i++ {
		req, reqErr := http.NewRequest("GET", requestURL, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, respErr := client.Do(req)
		if respErr != nil {
			err = respErr
			fmt.Printf("Attempt %d: failed to fetch data, error: %v\n", i+1, err)
			time.Sleep(retryDelay)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad status: %s", resp.Status)
			fmt.Printf("Attempt %d: received bad status, error: %v\n", i+1, err)
			time.Sleep(retryDelay)
			continue
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Attempt %d: failed to read response body, error: %v\n", i+1, err)
			time.Sleep(retryDelay)
			continue
		}
		return body, nil
	}
	return nil, fmt.Errorf("failed to fetch data after multiple attempts\n%s", err)
}

// joinStringsWithSlash 用斜杠连接字符串切片
func joinStringsWithSlash(strs []string) string {
	return strings.Join(strs, "/")
}

// getFirstPubdate 获取第一个出版日期
func getFirstPubdate(pubDates []string) string {
	if len(pubDates) > 0 {
		return pubDates[0]
	}
	return ""
}

// getCreator 获取创作者信息
func getCreator(directors []DoubanDirector) string {
	var names []string
	for _, director := range directors {
		names = append(names, director.Name)
	}
	return joinStringsWithSlash(names)
}