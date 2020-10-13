package crawl

import (
	"fmt"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

func (crawl *Crawl) startAPI() {
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = logInfo.Out

	r := gin.Default()

	pprof.Register(r)

	logInfo.Info("API server started")
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"rate":         crawl.URIsPerSecond.Rate(),
			"crawled":      crawl.Crawled.Value(),
			"queued":       crawl.Frontier.QueueCount.Value(),
			"running_time": fmt.Sprintf("%s", time.Since(crawl.StartTime)),
		})
	})

	r.Run(":9443")
}
