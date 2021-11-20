package photomgr

import (
	"fmt"
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

type PTT struct {
	//Inherit
	baseCrawler

	//Handle base folder address to store images
	BaseDir string
}

func NewPTT() *PTT {

	p := new(PTT)
	p.baseAddress = "https://www.ptt.cc"
	p.entryAddress = "https://www.ptt.cc/bbs/Beauty/index.html"
	return p
}

// GetAllFromURL: return all post images, like and dis in current page
func (p *PTT) GetAllFromURL(url string) (title string, allImages []string, like, dis int) {
	// Get https response with setting cookie over18=1
	resp := getResponseWithCookie(url)
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println(err)
		return title, allImages, like, dis
	}

	//Title and folder
	doc.Find(".article-metaline").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find(".article-meta-tag").Text(), "標題") {
			title = s.Find(".article-meta-value").Text()
		}
	})

	//all images
	foundImage := false
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		imgLink, _ := s.Attr("href")
		switch {
		case strings.Contains(imgLink, "https://i.imgur.com/"):
			allImages = append(allImages, imgLink)
			foundImage = true
		case strings.Contains(imgLink, "http://i.imgur.com/"):
			allImages = append(allImages, imgLink)
			foundImage = true
		case strings.Contains(imgLink, "https://pbs.twimg.com/"):
			allImages = append(allImages, imgLink)
			foundImage = true
		case strings.Contains(imgLink, "https://imgur.com/"):
			imgLink = imgLink + ".jpg"
			allImages = append(allImages, imgLink)
			foundImage = true
		}
	})

	if !foundImage {
		log.Println("Don't have any image in this article.")
	}

	//Like and Dislike
	doc.Find(".push-tag").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "推") {
			like++
		} else if strings.Contains(s.Text(), "噓") {
			dis++
		}
	})

	return title, allImages, like, dis
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

	//Title and folder
	articleTitle := ""
	doc.Find(".article-metaline").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find(".article-meta-tag").Text(), "標題") {
			articleTitle = s.Find(".article-meta-value").Text()
		}
	})
	dir := fmt.Sprintf("%v/%v - %v", p.BaseDir, "PTT", articleTitle)
	if exist, _ := exists(dir); exist {
		//fmt.Println("Already download")
		return
	}
	os.MkdirAll(filepath.FromSlash(dir), 0755)

	//Concurrecny
	linkChan := make(chan string)
	wg := new(sync.WaitGroup)
	for i := 0; i < workerNum; i++ {
		wg.Add(1)
		go p.worker(filepath.FromSlash(dir), linkChan, wg)
	}

	//Parse Image, currently support <IMG SRC> only
	foundImage := false
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		imgLink, _ := s.Attr("href")
		switch {
		case strings.Contains(imgLink, "https://i.imgur.com/"):
			linkChan <- imgLink
			foundImage = true
		case strings.Contains(imgLink, "http://i.imgur.com/"):
			linkChan <- imgLink
			foundImage = true
		case strings.Contains(imgLink, "https://pbs.twimg.com/"):
			linkChan <- imgLink
			foundImage = true
		case strings.Contains(imgLink, "https://imgur.com/"):
			imgLink = imgLink + ".jpg"
			linkChan <- imgLink
			foundImage = true
		}
	})

	if !foundImage {
		log.Println("Don't have any image in this article.")
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
		switch {
		case strings.Contains(imgLink, "https://i.imgur.com/"):
			ret = append(ret, imgLink)
			foundImage = true
		case strings.Contains(imgLink, "http://i.imgur.com/"):
			ret = append(ret, imgLink)
			foundImage = true
		case strings.Contains(imgLink, "https://pbs.twimg.com/"):
			ret = append(ret, imgLink)
			foundImage = true
		case strings.Contains(imgLink, "https://imgur.com/"):
			imgLink = imgLink + ".jpg"
			ret = append(ret, imgLink)
			foundImage = true
		}
	})

	if !foundImage {
		log.Println("Don't have any image in this article.")
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

//Set Ptt board psot number, fetch assigned (at least) number of posts. Return real number.
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

//Set Ptt board page index, fetch all post and return article count back
func (p *PTT) ParsePttPageByIndex(page int, replace bool) int {
	// Get https response with setting cookie over18=1
	resp := getResponseWithCookie(p.entryAddress)
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Fatal(err)
	}

	posts := make([]PostDoc, 0)

	maxPageNumberString := ""
	var PageWebSide string
	if page > 0 {
		// Find page result
		doc.Find(".btn-group a").Each(func(i int, s *goquery.Selection) {
			if strings.Contains(s.Text(), "上頁") {
				href, exist := s.Attr("href")
				if exist {
					targetString := strings.Split(href, "index")[1]
					targetString = strings.Split(targetString, ".html")[0]
					maxPageNumberString = targetString
				}
			}
		})
		pageNum, _ := strconv.Atoi(maxPageNumberString)
		pageNum = pageNum - page + 1
		PageWebSide = fmt.Sprintf("https://www.ptt.cc/bbs/Beauty/index%d.html", pageNum)
	} else {
		PageWebSide = p.entryAddress
	}

	// Get https response with setting cookie over18=1
	resp = getResponseWithCookie(PageWebSide)
	doc, err = goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find(".r-ent").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find(".title").Text())
		if CheckTitleWithBeauty(title) {
			likeCount, _ := strconv.Atoi(s.Find(".nrec span").Text())
			href, _ := s.Find(".title a").Attr("href")
			link := p.baseAddress + href
			newPost := PostDoc{
				ArticleID:    "",
				ArticleTitle: title,
				URL:          link,
				Likeint:      likeCount,
			}

			posts = append(posts, newPost)
		}
	})
	if replace {
		p.storedPost = posts
	} else {
		p.storedPost = append(p.storedPost, posts...)
	}

	return len(p.storedPost)
}

func getResponseWithCookie(url string) *http.Response {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("http failed:", err)
	}

	req.AddCookie(&http.Cookie{Name: "over18", Value: "1"})

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("client failed:", err)
	}
	return resp
}

func (p *PTT) GetPostLikeDis(target string) (int, int) {
	// Get https response with setting cookie over18=1
	resp := getResponseWithCookie(target)
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

// CheckTitleWithBeauty: check if title contains "美女" or "美女圖" or "美女圖片" or "美女圖片"
func CheckTitleWithBeauty(title string) bool {
	d, _ := regexp.MatchString("^\\[正妹\\].*", title)
	return d
}
