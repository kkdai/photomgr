package photomgr

import "testing"

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
