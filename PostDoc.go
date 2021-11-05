package photomgr

type PostDoc struct {
	ArticleID    string   `json:"article_id" bson:"article_id"`
	ArticleTitle string   `json:"article_title" bson:"article_title"`
	Author       string   `json:"author" bson:"author"`
	Date         string   `json:"date" bson:"date"`
	URL          string   `json:"url" bson:"url"`
	ImageLinks   []string `json:"image_links" bson:"image_links"`
	Likeint      int      `json:"likeint" bson:"likeint"`
	Dislikeint   int      `json:"dislikeint" bson:"dislikeint"`
}
