package photomgr

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	//To store current PTT post result
	storedPostURLList   []string
	storedPostTitleList []string
	storedStarList      []int
}

func NewPTT() *PTT {

	p := new(PTT)
	p.baseAddress = "https://www.ptt.cc"
	p.entryAddress = "https://www.ptt.cc/bbs/Beauty/index.html"
	return p
}

func (p *PTT) GetUrlPhotos(target string) []string {
	var resultSlice []string
	doc, err := goquery.NewDocument(target)
	if err != nil {
		log.Println(err)
		return nil
	}

	//Parse Image, currently support <IMG SRC> only
	doc.Find(".richcontent").Each(func(i int, s *goquery.Selection) {
		imgLink, exist := s.Find("img").Attr("src")
		if exist {
			resultSlice = append(resultSlice, "http:"+imgLink)
		}
	})
	return resultSlice
}

func (p *PTT) Crawler(target string, workerNum int) {
	doc, err := goquery.NewDocument(target)
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
		if strings.Contains(imgLink, "https://i.imgur.com/") {
			linkChan <- imgLink
			foundImage = true
		}
		if strings.Contains(imgLink, "https://imgur.com/") {
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

// Return parse page result count, it will be 0 if you still not parse any page
func (p *PTT) GetCurrentPageResultCount() int {
	return len(p.storedPostTitleList)
}

// Get post title by index in current parsed page
func (p *PTT) GetPostTitleByIndex(postIndex int) string {
	if postIndex >= len(p.storedPostTitleList) {
		return ""
	}
	return p.storedPostTitleList[postIndex]
}

// Get post URL by index in current parsed page
func (p *PTT) GetPostUrlByIndex(postIndex int) string {
	if postIndex >= len(p.storedPostURLList) {
		return ""
	}

	return p.storedPostURLList[postIndex]
}

// Get post like count by index in current parsed page
func (p *PTT) GetPostStarByIndex(postIndex int) int {
	if postIndex >= len(p.storedStarList) {
		return 0
	}
	return p.storedStarList[postIndex]
}

//Set Ptt board page index, fetch all post and return article count back
func (p *PTT) ParsePttPageByIndex(page int) int {
	doc, err := goquery.NewDocument(p.entryAddress)
	if err != nil {
		log.Fatal(err)
	}

	urlList := make([]string, 0)
	postList := make([]string, 0)
	starList := make([]int, 0)

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
					log.Println("total page:", targetString)
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

	doc, err = goquery.NewDocument(PageWebSide)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find(".r-ent").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find(".title").Text())
		likeCount, _ := strconv.Atoi(s.Find(".nrec span").Text())
		href, _ := s.Find(".title a").Attr("href")
		link := p.baseAddress + href
		urlList = append(urlList, link)
		log.Printf("%d:[%d★]%s\n", i, likeCount, title)
		starList = append(starList, likeCount)
		postList = append(postList, title)
	})

	// Print pages
	log.Printf("Pages: ")
	for i := page - 3; i <= page+2; i++ {
		if i >= 0 {
			if i == page {
				log.Printf("[%v] ", i)
			} else {
				log.Printf("%v ", i)
			}
		}
	}

	p.storedPostURLList = urlList
	p.storedStarList = starList
	p.storedPostTitleList = postList

	return len(p.storedPostTitleList)
}
