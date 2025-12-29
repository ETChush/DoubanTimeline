package douban_module

import (
	"fmt"
	"log"
	"strings"
)

// ExampleParseDoubanURL 示例：解析豆瓣URL
func ExampleParseDoubanURL() {
	// 测试不同类型的豆瓣链接
	urls := []string{
		"https://book.douban.com/subject/26752088/",   // 图书
		"https://movie.douban.com/subject/1292052/",  // 电影
		"https://www.douban.com/game/10734449/",      // 游戏
	}

	for _, url := range urls {
		fmt.Printf("Testing URL: %s\n", url)
		
		// 根据URL类型选择subjectType
		var subjectType string
		if strings.Contains(url, "book.douban.com") {
			subjectType = "book"
		} else if strings.Contains(url, "game/") {
			subjectType = "game"
		} else {
			subjectType = "movie"
		}

		// 解析URL
		subjectID, err := ParseDoubanURL(url, subjectType)
		if err != nil {
			log.Printf("Error parsing URL %s: %v\n", url, err)
			continue
		}

		fmt.Printf("  Subject Type: %s\n", subjectType)
		fmt.Printf("  Subject ID: %s\n", subjectID)
	}
}

// ExampleFetchDoubanMediaInfo 示例：获取豆瓣媒体信息
func ExampleFetchDoubanMediaInfo() {
	// 示例：获取电影信息
	movieURL := "https://movie.douban.com/subject/1292052/"
	subjectID, err := ParseDoubanURL(movieURL, "movie")
	if err != nil {
		log.Fatalf("Error parsing movie URL: %v", err)
	}

	fmt.Printf("Fetching info for movie ID: %s\n", subjectID)
	subject, err := FetchDoubanMediaInfo("movie", subjectID)
	if err != nil {
		log.Fatalf("Error fetching movie info: %v", err)
	}

	fmt.Printf("\nMovie Info:\n")
	fmt.Printf("Title: %s\n", subject.Title)
	fmt.Printf("Original Title: %s\n", subject.AltTitle)
	fmt.Printf("Director: %s\n", subject.Creator)
	fmt.Printf("Country: %s\n", subject.Press)
	fmt.Printf("Release Date: %s\n", subject.PubDate)
	fmt.Printf("Summary: %s\n", subject.Summary)
	fmt.Printf("Image URL: %s\n", subject.ImageURL)

	// 示例：获取图书信息
	bookURL := "https://book.douban.com/subject/26752088/"
	bookID, err := ParseDoubanURL(bookURL, "book")
	if err != nil {
		log.Fatalf("Error parsing book URL: %v", err)
	}

	fmt.Printf("\nFetching info for book ID: %s\n", bookID)
	bookSubject, err := FetchDoubanMediaInfo("book", bookID)
	if err != nil {
		log.Fatalf("Error fetching book info: %v", err)
	}

	fmt.Printf("\nBook Info:\n")
	fmt.Printf("Title: %s\n", bookSubject.Title)
	fmt.Printf("Subtitle: %s\n", bookSubject.AltTitle)
	fmt.Printf("Author: %s\n", bookSubject.Creator)
	fmt.Printf("Press: %s\n", bookSubject.Press)
	fmt.Printf("Pub Date: %s\n", bookSubject.PubDate)
	fmt.Printf("Summary: %s\n", bookSubject.Summary)
	fmt.Printf("Image URL: %s\n", bookSubject.ImageURL)
}

// ExampleCompleteWorkflow 示例：完整的工作流程
func ExampleCompleteWorkflow() {
	fmt.Println("=== 豆瓣数据获取完整工作流程示例 ===")
	
	// 用户输入的豆瓣链接
	userInputURL := "https://movie.douban.com/subject/1292052/"
	fmt.Printf("User provided URL: %s\n", userInputURL)

	// 1. 判断链接类型
	var subjectType string
	switch {
	case strings.Contains(userInputURL, "book.douban.com"):
		subjectType = "book"
	case strings.Contains(userInputURL, "www.douban.com/game/"):
		subjectType = "game"
	case strings.Contains(userInputURL, "movie.douban.com"):
		subjectType = "movie"
	default:
		log.Fatalf("Unsupported Douban URL type: %s", userInputURL)
	}

	fmt.Printf("Detected subject type: %s\n", subjectType)

	// 2. 解析URL获取subject ID
	subjectID, err := ParseDoubanURL(userInputURL, subjectType)
	if err != nil {
		log.Fatalf("Failed to parse URL: %v", err)
	}

	fmt.Printf("Extracted subject ID: %s\n", subjectID)

	// 3. 从豆瓣API获取数据
	mediaInfo, err := FetchDoubanMediaInfo(subjectType, subjectID)
	if err != nil {
		log.Fatalf("Failed to fetch media info: %v", err)
	}

	// 4. 使用获取到的数据
	fmt.Println("\n=== Fetched Media Info ===")
	fmt.Printf("Title: %s\n", mediaInfo.Title)
	if mediaInfo.AltTitle != "" {
		fmt.Printf("Alternative Title: %s\n", mediaInfo.AltTitle)
	}
	fmt.Printf("Creator: %s\n", mediaInfo.Creator)
	fmt.Printf("Press/Publisher: %s\n", mediaInfo.Press)
	fmt.Printf("Publication Date: %s\n", mediaInfo.PubDate)
	fmt.Printf("Summary: %s\n", truncateString(mediaInfo.Summary, 100)) // 截断长摘要
	fmt.Printf("Cover Image URL: %s\n", mediaInfo.ImageURL)
}

// 辅助函数：截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}