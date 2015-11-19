package ptt

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
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
	//Handle base folder address to store images
	BaseDir string

	//To store current PTT post result
	storedPostURLList   []string
	storedPostTitleList []string
	storedStarList      []int
}

func New() *PTT {
	return new(PTT)
}

const (
	basePttAddress = "https://www.ptt.cc"
	entryAddress   = "https://www.ptt.cc/bbs/Beauty/index.html"
)

var (
	threadId = regexp.MustCompile(`M.(\d*).`)
	imageId  = regexp.MustCompile(`([^\/]+)\.(png|jpg)`)
)

func (p *PTT) HasValidURL(url string) bool {
	return threadId.Match([]byte(url))
}

func worker(destDir string, linkChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for target := range linkChan {
		resp, err := http.Get(target)
		if err != nil {
			log.Printf("Http.Get\nerror: %s\ntarget: %s\n", err, target)
			continue
		}
		defer resp.Body.Close()

		m, _, err := image.Decode(resp.Body)
		if err != nil {
			log.Printf("image.Decode\nerror: %s\ntarget: %s\n", err, target)
			continue
		}

		// Ignore small images
		bounds := m.Bounds()
		if bounds.Size().X > 300 && bounds.Size().Y > 300 {
			imgInfo := imageId.FindStringSubmatch(target)
			finalPath := destDir + "/" + imgInfo[1] + "." + imgInfo[2]
			out, err := os.Create(filepath.FromSlash(finalPath))
			if err != nil {
				log.Printf("os.Create\nerror: %s\n", err)
				continue
			}
			defer out.Close()
			switch imgInfo[2] {
			case "jpg":
				jpeg.Encode(out, m, nil)
			case "png":
				png.Encode(out, m)
			}
		}
	}
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
	dir := fmt.Sprintf("%v/%v - %v", p.BaseDir, threadId.FindStringSubmatch(target)[1], articleTitle)
	os.MkdirAll(filepath.FromSlash(dir), 0755)

	//Concurrecny
	linkChan := make(chan string)
	wg := new(sync.WaitGroup)
	for i := 0; i < workerNum; i++ {
		wg.Add(1)
		go worker(filepath.FromSlash(dir), linkChan, wg)
	}

	//Parse Image, currently support <IMG SRC> only
	foundImage := false
	doc.Find(".richcontent").Each(func(i int, s *goquery.Selection) {
		imgLink, exist := s.Find("img").Attr("src")
		if exist {
			linkChan <- "http:" + imgLink
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
	doc, err := goquery.NewDocument(entryAddress)
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
		pageNum = pageNum - page
		PageWebSide = fmt.Sprintf("https://www.ptt.cc/bbs/Beauty/index%d.html", pageNum)
	} else {
		PageWebSide = entryAddress
	}

	doc, err = goquery.NewDocument(PageWebSide)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find(".r-ent").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find(".title").Text())
		likeCount, _ := strconv.Atoi(s.Find(".nrec span").Text())
		href, _ := s.Find(".title a").Attr("href")
		link := basePttAddress + href
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
