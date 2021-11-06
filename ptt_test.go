package photomgr

import "testing"

func TestGetPage(t *testing.T) {
	ptt := NewPTT()
	count := ptt.ParsePttPageByIndex(0, true)
	if count == 0 {
		t.Errorf("ParsePttPageByIndex: no content")
	}
}

func TestURLPhoto(t *testing.T) {
	ptt := NewPTT()
	title := ptt.GetPostTitleByIndex(5)
	if CheckTitleWithBeauty(title) {
		url := ptt.GetPostUrlByIndex(5)
		ret := ptt.GetUrlPhotos(url)
		if !ptt.HasValidURL(url) {
			t.Errorf("URLPhoto: URL is not correct")
		}

		if len(ret) == 0 {
			t.Errorf("URLPhoto: No result")
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
