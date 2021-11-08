package photomgr

import (
	"testing"
)

func TestGetPage(t *testing.T) {
	ptt := NewPTT()
	count := ptt.ParsePttPageByIndex(0, true)
	if count == 0 {
		t.Errorf("ParsePttPageByIndex: no content")
	}
}

func TestPageReplace(t *testing.T) {
	ptt := NewPTT()
	count1 := ptt.ParsePttPageByIndex(0, true)
	if count1 == 0 {
		t.Errorf("ParsePttPageByIndex: no content, p1=%d", count1)
	}
	count2 := ptt.ParsePttPageByIndex(1, true)
	if count2 == 0 {
		t.Errorf("ParsePttPageByIndex: no content, p2=%d", count2)
	}
	count11 := ptt.ParsePttPageByIndex(0, true)
	count3 := ptt.ParsePttPageByIndex(1, false)
	if count3 != count1+count2 {
		t.Errorf("ParsePttPageByIndex: replace failed. page 1:%d page 2: %d, total: %d", count11, count2, count3)
	}

}

func TestGetNumber(t *testing.T) {
	ptt := NewPTT()
	count := ptt.ParsePttByNumber(15, 0)
	if count < 15 {
		t.Errorf("TestGetNumber: error get number result, %d", count)
	}
}

func TestURLPhoto(t *testing.T) {
	ptt := NewPTT()
	ptt.ParsePttByNumber(6, 0)
	title := ptt.GetPostTitleByIndex(5)
	if CheckTitleWithBeauty(title) {
		url := ptt.GetPostUrlByIndex(5)
		ret := ptt.GetUrlPhotos(url)
		if !ptt.HasValidURL(url) {
			t.Errorf("TestURLPhoto: URL is not correct")
		}

		if len(ret) == 0 {
			t.Errorf("TestURLPhoto: No result")
		}
	}
}

func TestURLTitle(t *testing.T) {
	ptt := NewPTT()
	ptt.ParsePttByNumber(6, 0)
	title := ptt.GetPostTitleByIndex(5)
	if CheckTitleWithBeauty(title) {
		url := ptt.GetPostUrlByIndex(5)
		urlTitle := ptt.GetUrlTitle(url)
		if urlTitle == "" || !CheckTitleWithBeauty(urlTitle) {
			t.Errorf("TestURLTitle: title is not correct url_title=%s title=%s\n", urlTitle, title)
		}
	}
}

func TestURLLike(t *testing.T) {
	ptt := NewPTT()
	title := ptt.GetPostTitleByIndex(5)
	if CheckTitleWithBeauty(title) {
		url := ptt.GetPostUrlByIndex(5)
		if !ptt.HasValidURL(url) {
			t.Errorf("URLPhoto: URL is not correct")
		}

		like, dis := ptt.GetPostLikeDis(url)
		if like == 0 && dis == 0 {
			t.Errorf("like:%d, dis:%d", like, dis)
		}
	}
}

func TestUAllGirls(t *testing.T) {
	ptt := NewPTT()
	count := ptt.ParsePttPageByIndex(0, true)
	for i := 0; i < count; i++ {
		title := ptt.GetPostTitleByIndex(i)
		if CheckTitleWithBeauty(title) {
			url := ptt.GetPostUrlByIndex(i)
			if !ptt.HasValidURL(url) {
				t.Errorf("All Girl: %d post, title: %s \n", i, title)
			}
		}
	}
}
