package gee

import (
	"log"
	"time"
)

//中间件设计：插入点是框架接收到请求初始化Context对象后，允许用户使用自己定义的中间件做一些额外的处理
func Logger() HandlerFunc {
	return func(c *Context) {
		// 启动计时器
		t := time.Now()
		// 处理请求（中间件可等待执行其他的中间件或用户自己定义的 Handler处理结束后，再做一些额外的操作）
		c.Next()
		// time.Since(t)计算程序处理时间
		log.Printf("[%d] %s in %v", c.StatusCode, c.Req.RequestURI, time.Since(t))
	}
}
