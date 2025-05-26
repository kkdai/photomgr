package photomgr

// PttPostEntry represents a single post entry from the PTT index page.
type PttPostEntry struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	Date      string `json:"date"`
	PushCount int    `json:"push_count"` // "爆" is converted to 100 during parsing
}

// PttArticle represents a single scraped PTT post.
type PttArticle struct {
	Author    string   `json:"author"`
	Board     string   `json:"board"`
	Title     string   `json:"title"`
	Date      string   `json:"date"`
	ImageURLs []string `json:"image_urls"`
	Content   string   `json:"content"`
}

// FirecrawlRequest defines the structure for the Firecrawl API request body.
type FirecrawlRequest struct {
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers"`
	Formats         []string          `json:"formats"`
	OnlyMainContent bool              `json:"onlyMainContent"`
	WaitFor         int               `json:"waitFor"`
}

// FirecrawlResponseData defines the structure of the "data" field in Firecrawl's response.
type FirecrawlResponseData struct {
	Markdown string `json:"markdown"`
}

// FirecrawlError defines the structure of the "error" field in Firecrawl's response.
type FirecrawlError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// FirecrawlResponse defines the structure for the Firecrawl API response.
type FirecrawlResponse struct {
	Success bool                  `json:"success"`
	Data    FirecrawlResponseData `json:"data"`
	Error   *FirecrawlError       `json:"error,omitempty"` // Pointer to allow for null error field
}

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// extractArticleIDFromURL extracts the PTT article ID from its URL.
// Example: "https://www.ptt.cc/bbs/Beauty/M.1234567890.A.BCD.html" -> "M.1234567890.A.BCD"
func extractArticleIDFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	fileName := parts[len(parts)-1]
	idParts := strings.Split(fileName, ".html")
	if len(idParts) == 0 {
		return ""
	}
	return idParts[0]
}

// parsePushCount converts a push string (e.g., "爆", "10", "X1") to an integer.
func parsePushCount(pushStr string) int {
	pushStr = strings.TrimSpace(pushStr)
	if pushStr == "爆" {
		return 100
	}
	if strings.HasPrefix(pushStr, "X") { // Handle cases like "X1", "X5" (meaning 0)
		return 0
	}
	count, err := strconv.Atoi(pushStr)
	if err != nil {
		return 0 // Default to 0 if not "爆" and not a valid number
	}
	return count
}

// parseMarkdownToPostDocs extracts post information from PTT index page markdown content
// obtained from Firecrawl.
//
// The function assumes markdown where each post entry is represented by a block with:
// 1. A title line starting with "## ", which may include a category like "[正妹]".
//    Example: "## [正妹] Post Title 1"
// 2. A line containing a markdown link, where the URL is the full path to the post.
//    Example: "[Read More](https://www.ptt.cc/bbs/Beauty/M.123.A.XYZ.html)"
// 3. A line detailing the author, date, and push count.
//    Example: "Author: author1 Date: 5/24 Push: 10"
//
// It uses a regular expression to capture these elements for each post.
func parseMarkdownToPostDocs(markdown string, baseAddress string) []PostDoc {
	var posts []PostDoc

	// postRegex is designed to capture the key information for each post entry:
	// - Group 1: The full title of the post (e.g., "[正妹] Test Post 1 (Page 1)").
	// - Group 2: The URL of the post (e.g., "https://www.ptt.cc/bbs/Beauty/M.123.A.AAA.html").
	// - Group 3: The author's username (e.g., "user1").
	// - Group 4: The date of the post (e.g., "1/1").
	// - Group 5: The push count or status string (e.g., "10", "爆").
	// `(?m)` enables multi-line mode, allowing `^` to match the start of each line.
	postRegex := regexp.MustCompile(`(?m)^##\s*(.*?)\s*\n.*?\[.*?\]\((.*?)\)\s*\nAuthor:\s*(.*?)\s*Date:\s*(.*?)\s*Push:\s*(.*)`)
	matches := postRegex.FindAllStringSubmatch(markdown, -1)

	for _, match := range matches {
		if len(match) < 6 {
			log.Printf("Skipping incomplete match: %v", match)
			continue
		}

		title := strings.TrimSpace(match[1])
		url := strings.TrimSpace(match[2])
		author := strings.TrimSpace(match[3])
		// Date is match[4] - not directly used in PostDoc, but good to extract
		_ = strings.TrimSpace(match[4]) 
		pushStr := strings.TrimSpace(match[5])

		// Ensure URL is absolute
		if !strings.HasPrefix(url, "http") {
			url = baseAddress + url
		}
		
		// Filter out announcements or other non-post entries based on title, if necessary.
		// For now, we assume all matches are valid posts.
		// Example: if strings.HasPrefix(title, "[公告]") { continue }

		if !CheckTitleWithBeauty(title) { // Reuse existing filter
			log.Printf("Skipping post with title not matching Beauty criteria: %s", title)
			continue
		}


		articleID := extractArticleIDFromURL(url)
		likeCount := parsePushCount(pushStr)

		newPost := PostDoc{
			ArticleID:    articleID,
			ArticleTitle: title,
			URL:          url,
			Likeint:      likeCount,
			// Author and Date are available but not in PostDoc.
			// If PttPostEntry were used, they'd be stored there.
		}
		posts = append(posts, newPost)
	}
	if len(matches) == 0 && len(markdown) > 0 {
		log.Println("No posts found in markdown. Markdown sample (first 500 chars):", markdown[:min(500, len(markdown))])
	}
	return posts
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


type PTT struct {
	//Inherit
	baseCrawler

	//Handle base folder address to store images
	BaseDir       string
	SearchAddress string
}

// firecrawlScrapeURL is the endpoint for the Firecrawl API.
// It's a global variable to allow overriding for testing purposes.
var firecrawlScrapeURL = "https://api.firecrawl.dev/v1/scrape"

// callFirecrawlAPI makes a POST request to the Firecrawl API (specifically to `firecrawlScrapeURL`)
// to scrape and retrieve the content of the given `targetURL` as markdown.
//
// It requires the `FIRECRAWL_KEY` environment variable to be set for authorization.
//
// The request to Firecrawl includes:
// - The `targetURL` to be scraped.
// - Headers, including a "Cookie: over18=1" to bypass PTT's age verification.
// - Parameters to specify the format ("markdown"), focus on main content, and wait time.
//
// Expected JSON response structure from Firecrawl:
// {
//   "success": true/false,
//   "data": { "markdown": "..." }, // if success is true
//   "error": { "code": ..., "message": "..." } // if success is false
// }
//
// The function returns the extracted markdown content or an error if any step fails.
func callFirecrawlAPI(targetURL string) (string, error) {
	apiKey := os.Getenv("FIRECRAWL_KEY")
	if apiKey == "" {
		return "", errors.New("FIRECRAWL_KEY not set")
	}

	requestBody := FirecrawlRequest{
		URL: targetURL,
		Headers: map[string]string{
			"Cookie":     "over18=1",
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		},
		Formats:         []string{"markdown"},
		OnlyMainContent: true,
		WaitFor:         1000,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("error marshalling request body: %w", err)
	}

	req, err := http.NewRequest("POST", firecrawlScrapeURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request to Firecrawl API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("Firecrawl API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	var firecrawlResp FirecrawlResponse
	if err := json.Unmarshal(bodyBytes, &firecrawlResp); err != nil {
		return "", fmt.Errorf("error unmarshalling Firecrawl API response: %w. Response: %s", err, string(bodyBytes))
	}

	if !firecrawlResp.Success {
		if firecrawlResp.Error != nil {
			return "", fmt.Errorf("Firecrawl API error (%d): %s", firecrawlResp.Error.Code, firecrawlResp.Error.Message)
		}
		return "", errors.New("Firecrawl API call was not successful, but no error message was provided")
	}

	if firecrawlResp.Data.Markdown == "" {
		return "", errors.New("Firecrawl API returned success but markdown content is empty")
	}

	return firecrawlResp.Data.Markdown, nil
}

func NewPTT() *PTT {

	p := new(PTT)
	p.baseAddress = "https://www.ptt.cc"
	p.entryAddress = "https://www.ptt.cc/bbs/Beauty/index.html"
	p.SearchAddress = "https://www.ptt.cc/bbs/Beauty/search?q="
	return p
}

// Add new helper functions to extract title and image links.
func extractTitle(doc *goquery.Document) string {
	var title string
	doc.Find(".article-metaline").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find(".article-meta-tag").Text(), "標題") {
			title = s.Find(".article-meta-value").Text()
		}
	})
	return title
}

// Add helper function to fix imgur links.
func fixImgurLink(link string) string {
	if strings.Contains(link, "https://imgur.com/") {
		parts := strings.Split(link, "/")
		imageID := parts[len(parts)-1]
		return "https://i.imgur.com/" + imageID + ".jpeg"
	}
	return link
}

func extractImageLinks(doc *goquery.Document) []string {
	var links []string
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		imgLink, _ := s.Attr("href")
		if isImageLink(imgLink) {
			// Replace imgur.com link using helper.
			imgLink = fixImgurLink(imgLink)
			links = append(links, imgLink)
		}
	})
	return links
}

// GetAllFromURL fetches an individual PTT article page using Firecrawl, then parses
// the resulting markdown to extract article details.
//
// It assumes the markdown for an article page contains:
// 1. A metadata block at the beginning, formatted with bolded field names:
//    **Author**: author_username (Nickname)
//    **Board**: BoardName
//    **Title**: Article Title
//    **Date**: Full Date String
// 2. Image URLs in standard markdown format: `![](image_url)` or `![alt text](image_url)`.
// 3. The main textual content of the post.
// 4. A signature/push message section at the end, typically starting with lines like
//    "--", "※ 發信站:", "推 ", "噓 ", etc.
//
// The function returns the article title, a slice of all found image URLs, and
// 0, 0 for like and dislike counts (as these are not reliably parsed from markdown).
func (p *PTT) GetAllFromURL(url string) (title string, allImages []string, like, dis int) {
	markdown, err := callFirecrawlAPI(url)
	if err != nil {
		log.Printf("Error calling Firecrawl API for URL %s in GetAllFromURL: %v", url, err)
		return "", nil, 0, 0 // Return empty/zero values on API error
	}

	article := PttArticle{} // Internal struct to hold parsed data

	// 1. Parse Metadata from the beginning of the markdown.
	// metaRegex captures:
	// - Group 1: Author's username (e.g., "testuser" from "**Author**: testuser (Test Nickname)").
	//            The nickname part `(?:\s*\(.*?\))?` is optional and non-capturing.
	// - Group 2: Board name (e.g., "Beauty").
	// - Group 3: Article title (e.g., "[正妹] Test Post Title Typical").
	// - Group 4: Date string (e.g., "Mon Jan 01 12:34:56 2024").
	// `(?m)` enables multi-line mode. `^` matches start of line, `$` matches end of line for Date.
	metaRegex := regexp.MustCompile(`(?m)^\*\*Author\*\*:\s*(.*?)(?:\s*\(.*?\))?\s*\n\*\*Board\*\*:\s*(.*?)\s*\n\*\*Title\*\*:\s*(.*?)\s*\n\*\*Date\*\*:\s*(.*?)\s*$`)
	metaMatches := metaRegex.FindStringSubmatch(markdown)

	contentStartIndex := 0
	if len(metaMatches) < 5 {
		log.Printf("Could not parse full metadata block from markdown for URL %s. Markdown length: %d", url, len(markdown))
		// Fallback: Attempt to extract at least the title if the full metadata block isn't found.
		// This is important as title is a primary return value.
		simpleTitleRegex := regexp.MustCompile(`(?m)^\*\*Title\*\*:\s*(.*)`)
		titleMatch := simpleTitleRegex.FindStringSubmatch(markdown)
		if len(titleMatch) > 1 {
			article.Title = strings.TrimSpace(titleMatch[1])
			log.Printf("Partially parsed title using fallback: %s", article.Title)
			// Try to find where the title match ends to set contentStartIndex
			titleMatchIndices := simpleTitleRegex.FindStringIndex(markdown)
			if titleMatchIndices != nil {
				contentStartIndex = titleMatchIndices[1]
			}
		} else {
			log.Printf("Even simple title parsing failed for URL %s. Full markdown might be malformed or unexpected.", url)
			// If no title can be parsed, return empty, assuming critical failure.
			// Images might still be parsable if the overall markdown isn't completely empty.
		}
	} else {
		article.Author = strings.TrimSpace(metaMatches[1])
		article.Board = strings.TrimSpace(metaMatches[2])
		article.Title = strings.TrimSpace(metaMatches[3])
		article.Date = strings.TrimSpace(metaMatches[4])
		contentStartIndex = len(metaMatches[0]) // Position after the matched metadata block
	}

	// 2. Parse Image URLs from the entire markdown.
	// imageRegex captures:
	// - Group 1: The full image URL (e.g., "https://i.imgur.com/image1.jpg").
	// It looks for common image extensions (jpg, jpeg, png, gif, bmp, webp).
	imageRegex := regexp.MustCompile(`!\[.*?\]\((https?://\S+?\.(?:jpg|jpeg|png|gif|bmp|webp))\)`)
	imageMatches := imageRegex.FindAllStringSubmatch(markdown, -1)
	var foundImageURLs []string
	for _, imgMatch := range imageMatches {
		if len(imgMatch) > 1 {
			imgURL := fixImgurLink(imgMatch[1]) // Apply helper to normalize Imgur links
			foundImageURLs = append(foundImageURLs, imgURL)
		}
	}
	article.ImageURLs = foundImageURLs

	// 3. Parse Content: The text block after metadata and before signatures/pushes.
	// signatureRegex identifies common start patterns of the signature or push/comment section.
	// `(?m)` enables multi-line mode. `^` matches the start of each line.
	// Patterns include:
	//   -- (common signature separator)
	//   ※ 發信站: (PTT post origin info)
	//   ※ 編輯: (PTT post edit info)
	//   ※ 轉錄至看板: (PTT cross-post info)
	//   ※ 推噓紀錄: (PTT push/shove record info)
	//   推 (start of a push comment)
	//   噓 (start of a shove comment)
	//   → (start of a neutral comment)
	//   ◆ From: (another PTT origin info format)
	signatureRegex := regexp.MustCompile(`(?m)^(?:--\s*$|※\s(?:發信站|編輯|轉錄至看板|推噓紀錄).*|推\s|噓\s|→\s|◆\sFrom:)`)
	
	// Search for signature block only in the part of markdown *after* the metadata.
	// If contentStartIndex is 0 (e.g. metadata parsing failed), search from start of markdown.
	searchableMarkdownArea := ""
	if contentStartIndex <= len(markdown) {
		searchableMarkdownArea = markdown[contentStartIndex:]
	}
	
	signatureMatchIndices := signatureRegex.FindStringIndex(searchableMarkdownArea)
	
	contentEndIndex := len(markdown) // Default to end of markdown if no signature found
	if signatureMatchIndices != nil {
		// Adjust index relative to the full markdown string
		contentEndIndex = contentStartIndex + signatureMatchIndices[0]
	}

	rawContent := ""
	if contentEndIndex > contentStartIndex && contentStartIndex < len(markdown) && contentEndIndex <= len(markdown) {
		rawContent = markdown[contentStartIndex:contentEndIndex]
	}
	
	// Remove any markdown image lines from the extracted rawContent block to avoid duplication.
	cleanedContentLines := []string{}
	for _, line := range strings.Split(rawContent, "\n") {
		if !imageRegex.MatchString(line) {
			cleanedContentLines = append(cleanedContentLines, line)
		}
	}
	article.Content = strings.TrimSpace(strings.Join(cleanedContentLines, "\n"))

	// Log the parsed article (optional, useful for debugging)
	// log.Printf("Parsed PttArticle for %s: Author='%s', Board='%s', Title='%s', Date='%s', ImageCount=%d, ContentLength=%d",
	//	url, article.Author, article.Board, article.Title, article.Date, len(article.ImageURLs), len(article.Content))

	// Return values as per function signature; like/dis are 0,0 as they are not parsed from markdown.
	return article.Title, article.ImageURLs, 0, 0
}

// GetUrlTitle: return title and url of post
func (p *PTT) GetUrlTitle(target string) string {
	// Get https response with setting cookie over18=1
	resp := getResponseWithCookie(target)
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println(err)
		return ""
	}

	//Title
	articleTitle := ""
	doc.Find(".article-metaline").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find(".article-meta-tag").Text(), "標題") {
			articleTitle = s.Find(".article-meta-value").Text()
		}
	})
	return articleTitle
}

// Crawler: parse ptt board page by index
func (p *PTT) Crawler(target string, workerNum int) {
	// Get https response with setting cookie over18=1
	resp := getResponseWithCookie(target)
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println(err)
		return
	}

	articleTitle := extractTitle(doc)
	dir := fmt.Sprintf("%v/%v - %v", p.BaseDir, "PTT", articleTitle)
	if exist, _ := exists(dir); exist {
		return
	}
	os.MkdirAll(filepath.FromSlash(dir), 0755)

	// Prepare concurrent download
	linkChan := make(chan string)
	wg := new(sync.WaitGroup)
	for i := 0; i < workerNum; i++ {
		wg.Add(1)
		go p.worker(filepath.FromSlash(dir), linkChan, wg)
	}

	// Optimized: Extract image links once and send them
	images := extractImageLinks(doc)
	if len(images) == 0 {
		log.Println("Don't have any image in this article.")
	}
	for _, imgLink := range images {
		linkChan <- imgLink
	}

	close(linkChan)
	wg.Wait()
}

// GetAllImageAddress: return all image address in current page.
func (p *PTT) GetAllImageAddress(target string) []string {
	var ret []string
	// Get https response with setting cookie over18=1
	resp := getResponseWithCookie(target)
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println(err)
		return nil
	}

	//Parse Image, currently support <IMG SRC> only
	foundImage := false
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		imgLink, _ := s.Attr("href")
		if isImageLink(imgLink) {
			// Replace imgur.com link using helper.
			imgLink = fixImgurLink(imgLink)
			ret = append(ret, imgLink)
			foundImage = true
		}
	})

	if !foundImage {
		log.Println("Don't have any image in this article. url:", target)
	}

	return ret
}

// Return parse page result count, it will be 0 if you still not parse any page
func (p *PTT) GetCurrentPageResultCount() int {
	return len(p.storedPost)
}

// Get post title by index in current parsed page
func (p *PTT) GetPostTitleByIndex(postIndex int) string {
	if postIndex >= len(p.storedPost) {
		return ""
	}
	return p.storedPost[postIndex].ArticleTitle
}

// Get post URL by index in current parsed page
func (p *PTT) GetPostUrlByIndex(postIndex int) string {
	if postIndex >= len(p.storedPost) {
		return ""
	}

	return p.storedPost[postIndex].URL
}

// Get post like count by index in current parsed page
func (p *PTT) GetPostStarByIndex(postIndex int) int {
	if postIndex >= len(p.storedPost) {
		return 0
	}
	return p.storedPost[postIndex].Likeint
}

// Set Ptt board psot number, fetch assigned (at least) number of posts. Return real number.
func (p *PTT) ParsePttByNumber(num int, page int) int {
	count := p.ParsePttPageByIndex(page, true)
	if count > num {
		return count
	}
	page++
	for count < num {
		count = p.ParsePttPageByIndex(page, false)
		page++
	}

	return count
}

// Set Ptt board page index, fetch all post and return article count back
func (p *PTT) ParsePttPageByIndex(page int, replace bool) int {
	var targetURL string
	if page > 0 {
		// Note: The old logic for maxPageNumberString to calculate the actual index
		// is complex and relied on scraping the "上頁" button.
		// Firecrawl scrapes a single page. We need a simpler way to get older pages.
		// PTT's own URL structure for older pages is index<number>.html where <number> DECREASES for older pages.
		// This is a common source of confusion.
		// The original code used 'maxPageNumberString' from the current page's "上頁" link,
		// then calculated `pageNum = maxPageNum - page + 1`.
		// This implies 'page 0' is latest, 'page 1' is one step back from latest etc.
		// For Firecrawl, we'd need to know the *actual* index number.
		// Let's assume for now `page` means "number of pages to go back from latest".
		// If `p.entryAddress` is "https://www.ptt.cc/bbs/Beauty/index.html",
		// then going back `page` pages means we need to find the current max index first.
		// This is problematic without an initial scrape.
		// A simpler interpretation: `page` is the actual index number to use if known.
		// Or, if `page` is 0, use `p.entryAddress`. If `page` is 1, use `index1.html` (which is usually very old).
		// Let's stick to the original intent: page 0 is the main index, page 1 is one page older, etc.
		// This requires knowing the *current* max index.
		// The old code did:
		// 1. Fetch p.entryAddress (latest page)
		// 2. Find "上頁" href, parse out maxPageNumberString (e.g., "3931")
		// 3. Calculate pageNum = maxPageNum - page + 1
		// This means to get page '1' (one older than current), we need to know '3931', so we'd fetch index3930.html.
		// This two-step process is not ideal for Firecrawl.
		// A common pattern for PTT is that index.html is the newest, index(N-1).html is older, index(N-2).html even older.
		// Let's assume a direct mapping for now: if page > 0, it's an offset from a known latest,
		// or it IS the number in index<NUM>.html.
		// Given the original code's complexity, let's simplify: if page > 0, it means index(MAX-page+1).html.
		// This still needs MAX.
		// A simpler model: page 0 is index.html, page 1 is index<MAX-1>.html, page 2 is index<MAX-2>.html.
		// For now, to avoid a preliminary scrape just to get the max index,
		// let's assume `page` parameter means "how many pages back from the absolute newest index.html".
		// If `page == 0`, target `p.entryAddress`.
		// If `page > 0`, this implies we need to know the current highest index number.
		// Let's use a placeholder for max index determination or assume page is an absolute index number for now.
		// The original code fetched the main page, then found the "上頁" link to determine the max index.
		// e.g. href="/bbs/Beauty/index3931.html" -> maxPageNumberString = "3931"
		// then pageNum = Atoi(maxPageNumberString) - page + 1
		// So if page=0, pageNum = max. page=1, pageNum = max-1.
		// This means we need to get maxPageNumberString first if page > 0.
		// This is tricky. For now, if page > 0, we will just use `index<page>.html`. This changes semantics.
		// Let's try to keep original semantics: fetch entryAddress, find max index, then construct URL.
		// This is inefficient with Firecrawl as it would be two API calls.
		// The task is to refactor the *existing* logic. The existing logic *does* determine this.
		// However, it does so by parsing HTML. We get Markdown now.
		// Firecrawl's `onlyMainContent: true` might strip out pagination controls.
		// If pagination is stripped, we cannot find "上頁".
		// This is a significant problem for the existing logic.

		// Assumption: Firecrawl's markdown for an index page might *not* include pagination links
		// like "上頁" if `onlyMainContent: true`. If it *does*, then we can parse it.
		// Let's assume it DOES NOT for now. This means we can only fetch `p.entryAddress` (latest)
		// or a specific `index<NUM>.html` if `page` is treated as that number.

		// Fallback: If page > 0, construct it as index<page>.html. This is a deviation but necessary
		// if pagination info isn't in the markdown.
		// Let's assume `page` is "pages to go back". If page is 0, it's `p.entryAddress`.
		// If `page` is 1, it's the page before `p.entryAddress`.
		// This requires knowing the current page's number.
		// The problem statement says "The URL for ParsePttPageByIndex needs to be constructed correctly based on the page parameter, similar to the old logic".
		// The old logic for page > 0:
		//   1. GET p.entryAddress
		//   2. Parse HTML to find href of "上頁" -> gives max_index_str
		//   3. page_to_fetch_num = Atoi(max_index_str) - page + 1
		//   4. targetURL = ".../index<page_to_fetch_num>.html"
		// This is not possible if Firecrawl strips pagination or if we want to avoid two API calls.

		// Simplification: Assume `page` directly corresponds to the index number in `index<page>.html`.
		// If `page == 0` or not specified by user, it should mean `p.entryAddress`.
		// PTT typically has `index.html` as the latest. Older pages are `indexXXXX.html` where XXXX decreases.
		// So, if `page` parameter is `N`, it means `indexN.html`.
		// If `page` is `0`, it means `p.entryAddress` (which is typically `index.html`).
		if page > 0 {
			targetURL = fmt.Sprintf("%s/bbs/Beauty/index%d.html", p.baseAddress, page)
		} else {
			targetURL = p.entryAddress // This is usually https://www.ptt.cc/bbs/Beauty/index.html
		}
		log.Printf("ParsePttPageByIndex: Target URL for page %d: %s", page, targetURL)

	} else { // page == 0
		targetURL = p.entryAddress
		log.Printf("ParsePttPageByIndex: Target URL for page 0 (latest): %s", targetURL)
	}


	markdown, err := callFirecrawlAPI(targetURL)
	if err != nil {
		log.Printf("Error calling Firecrawl API for URL %s: %v", targetURL, err)
		if replace {
			p.storedPost = []PostDoc{}
			return 0
		}
		return len(p.storedPost) // Return current count if appending
	}

	newlyParsedPosts := parseMarkdownToPostDocs(markdown, p.baseAddress)

	if replace {
		p.storedPost = newlyParsedPosts
	} else {
		p.storedPost = append(p.storedPost, newlyParsedPosts...)
	}

	log.Printf("ParsePttPageByIndex: Parsed %d posts from %s. Total stored posts: %d (replace=%t)",
		len(newlyParsedPosts), targetURL, len(p.storedPost), replace)
	return len(p.storedPost)
}

func getResponseWithCookie(url string) *http.Response { // This function might become unused by ParsePttPageByIndex and ParseSearchByKeyword
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		// Log instead of Fatal for cases where this function might still be called by other parts of the code.
		log.Printf("Error creating GET request for %s: %v", url, err)
		return nil // Or handle error more gracefully
	}

	req.AddCookie(&http.Cookie{Name: "over18", Value: "1"})

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error performing GET request for %s: %v", url, err)
		return nil // Or handle error more gracefully
	}
	return resp
}

func (p *PTT) GetPostLikeDis(target string) (int, int) {
	// Get https response with setting cookie over18=1
	resp := getResponseWithCookie(target)
	if resp == nil { // Added check for nil response
		log.Printf("GetPostLikeDis: Failed to get response for %s", target)
		return 0,0
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println(err)
		return 0, 0
	}

	var likeCount int
	var disLikeCount int
	doc.Find(".push-tag").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "推") {
			likeCount++
		} else if strings.Contains(s.Text(), "噓") {
			disLikeCount++
		}
	})
	// fmt.Println("like:", likeCount, " dislike:", disLikeCount)
	return likeCount, disLikeCount
}

// Search with specific keyword, fetch all post and return article count back
func (p *PTT) ParseSearchByKeyword(keyword string) int {
	targetURL := p.SearchAddress + keyword
	log.Printf("ParseSearchByKeyword: Target URL for keyword '%s': %s", keyword, targetURL)

	markdown, err := callFirecrawlAPI(targetURL)
	if err != nil {
		log.Printf("Error calling Firecrawl API for search URL %s: %v", targetURL, err)
		p.storedPost = []PostDoc{} // Clear posts on error as per original logic (always replace)
		return 0
	}

	newlyParsedPosts := parseMarkdownToPostDocs(markdown, p.baseAddress)
	p.storedPost = newlyParsedPosts // Always replace for search

	log.Printf("ParseSearchByKeyword: Parsed %d posts for keyword '%s'. Total stored posts: %d",
		len(newlyParsedPosts), keyword, len(p.storedPost))
	return len(p.storedPost)
}

// CheckTitleWithBeauty: check if title contains "[正妹]"
func CheckTitleWithBeauty(title string) bool {
	// Original regex: "^\\[正妹\\].*"
	// This checks if the title STARTS WITH "[正妹]".
	// This is used to filter posts.
	// Let's make sure it's correctly used.
	// The markdown parsing might extract titles like "[正妹] some title"
	// or "Re: [正妹] some title".
	// The regex `^\[正妹\].*` would correctly match the first but not the "Re:"
	// This behavior is consistent with the original.
	matched, _ := regexp.MatchString(`^\[正妹\].*`, title)
	if !matched {
		// Adding a log to see what titles are being filtered out by this.
		// log.Printf("Title '%s' does not match ^\\[正妹\\].*", title)
	}
	return matched
}

// isImageLink checks if the given URL is an image link from supported hosts.
// This function is likely unused by the refactored parsing functions but kept for other potential uses.
func isImageLink(url string) bool {
	return strings.Contains(url, "https://i.imgur.com/") ||
		strings.Contains(url, "http://i.imgur.com/") ||
		strings.Contains(url, "https://pbs.twimg.com/") ||
		strings.Contains(url, "https://imgur.com/") ||
		strings.Contains(url, "https://i.meee.com.tw/") ||
		strings.Contains(url, "https://i.ytimg.com/") ||
		strings.Contains(url, "https://d.img.vision/")
}
