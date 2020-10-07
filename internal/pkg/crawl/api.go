package crawl

import (
	"github.com/gin-gonic/gin"
)

func (crawl *Crawl) StartAPI() {
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"rate":    crawl.URLsPerSecond.Rate(),
			"crawled": crawl.Crawled.Value(),
			"queued":  crawl.Frontier.QueueCount.Value(),
		})
	})

	r.Run(":9443")
}
