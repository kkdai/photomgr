package photomgr

import (
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type baseCrawler struct {

	//Init on inherit class
	baseAddress  string
	entryAddress string

	// //To store current baseCrawler post result
	storedPost []PostDoc
}

var (
	threadId = regexp.MustCompile(`M.(\d*).`)
	// Updated to include jpeg extension.
	imageId  = regexp.MustCompile(`([^\/]+)\.(png|jpg|jpeg)`)
)

func (b *baseCrawler) HasValidURL(url string) bool {
	return threadId.Match([]byte(url))
}

// Return parse page result count, it will be 0 if you still not parse any page
func (b *baseCrawler) GetCurrentPageResultCount() int {
	return len(b.storedPost)
}

// Get post title by index in current parsed page
func (b *baseCrawler) GetPostTitleByIndex(postIndex int) string {
	if postIndex >= len(b.storedPost) {
		return ""
	}
	return b.storedPost[postIndex].ArticleTitle
}

// Get post URL by index in current parsed page
func (b *baseCrawler) GetPostUrlByIndex(postIndex int) string {
	if postIndex >= len(b.storedPost) {
		return ""
	}

	return b.storedPost[postIndex].URL
}

// Get post like count by index in current parsed page
func (b *baseCrawler) GetPostStarByIndex(postIndex int) int {
	if postIndex >= len(b.storedPost) {
		return 0
	}
	return b.storedPost[postIndex].Likeint
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func (b *baseCrawler) worker(destDir string, linkChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for target := range linkChan {
		req, err := http.NewRequest("GET", target, nil)
		if err != nil {
			log.Printf("http.NewRequest error: %s, target: %s", err, target)
			continue
		}
		// Set User-Agent header
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		// Set Referer header if target is from i.imgur.com
		if strings.Contains(target, "i.imgur.com") {
			re := regexp.MustCompile(`([^\/]+)\.(jpg|jpeg|png)`)
			matches := re.FindStringSubmatch(target)
			if len(matches) >= 2 {
				req.Header.Set("Referer", "https://imgur.com/"+matches[1])
			}
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("client.Do error: %s, target: %s", err, target)
			continue
		}
		defer resp.Body.Close()

		m, _, err := image.Decode(resp.Body)
		if err != nil {
			m, err = png.Decode(resp.Body)
			if err != nil {
				log.Printf("image.Decode error: %s, target: %s", err, target)
				continue
			}
		}

		// Ignore small images
		bounds := m.Bounds()
		if bounds.Size().X > 300 && bounds.Size().Y > 300 {
			imgInfo := imageId.FindStringSubmatch(target)
			if len(imgInfo) < 3 {
				log.Printf("imageId regex did not match target: %s", target)
				continue
			}
			ext := imgInfo[2]
			if ext == "jpeg" {
				ext = "jpg"
			}
			finalPath := destDir + "/" + imgInfo[1] + "." + ext
			out, err := os.Create(filepath.FromSlash(finalPath))
			if err != nil {
				log.Printf("os.Create error: %s", err)
				continue
			}
			defer out.Close()
			switch ext {
			case "jpg":
				jpeg.Encode(out, m, nil)
			case "png":
				png.Encode(out, m)
			case "gif":
				gif.Encode(out, m, nil)
			}
		}
	}
}
