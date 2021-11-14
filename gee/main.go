package main

import (
	"fmt"
	"net/http"

	"html/template"
	"time"

	"gee"
)


func FormatAsDate(t time.Time) string {
	year, month, day := t.Date()
	return fmt.Sprintf("%d-%02d-%02d", year, month, day)
}
func main() {
	r := gee.Default()
	r.SetFuncMap(template.FuncMap{   //自定义渲染函数
		"FormatAsDate": FormatAsDate,
	})
	r.LoadHTMLGlob("templates/*")  //模板解析
	r.Static("/assets", "./static")

	//定义Group来创建一个新的RouterGroup
	// Engine和RouterGroup对象都可以访问，因为它们相互继承
	v1 := r.Group("/v1")
	{
		v1.POST("/login", func(c *gee.Context) {
			c.JSON(http.StatusOK, gee.H{
				"username": c.PostForm("username"),
				"password": c.PostForm("password"),
			})
		})

	}

	//html模板引用
	r.GET("/date", func(c *gee.Context) {
		c.HTML(http.StatusOK, "custom_func.tmpl", gee.H{
			"title": "gee",
			"now":   time.Date(2000, 11, 1, 0, 0, 0, 0, time.UTC),
		})
	})
	r.Run(":8000") //在这里构造了一个 Context 对象
}
