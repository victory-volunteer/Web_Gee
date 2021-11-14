package gee

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type H map[string]interface{}

type Context struct {
	Writer http.ResponseWriter
	Req    *http.Request
	Path   string
	Method string
	Params map[string]string //解析后的路由参数
	StatusCode int
	// middleware
	//需要在Context中保存,因为在设计中，中间件不仅作用在处理流程前，
	//也可以作用在处理流程后，即在用户定义的 Handler 处理完毕后，还可以执行剩下的操作
	handlers []HandlerFunc //保存所有路由函数及中间件（注册中间件其实就是将中间件函数追加到handlers中）
	index    int
	engine *Engine //通过 Context 访问 Engine 中的 HTML 模板
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Path:   req.URL.Path,
		Method: req.Method,
		Req:    req,
		Writer: w,
		index:  -1, //记录当前执行到第几个中间件
	}
}

//在中间件中调用Next方法时，控制权交给了下一个中间件，直到调用到最后一个中间件，然后再从后往前，调用每个中间件在Next方法之后定义的部分
//不是所有的handler（例如Logger()调用了Next()）都会调用 Next(),手工调用 Next()，一般用于在请求前后各实现一些行为。
//如果中间件只作用于请求前，可以省略调用Next()，兼容性比较好
func (c *Context) Next() {
	c.index++
	s := len(c.handlers)
	for ; c.index < s; c.index++ {
		c.handlers[c.index](c) //执行注册的处理方法
	}
}

func (c *Context) Fail(code int, err string) {
	c.index = len(c.handlers) //这是短路中间件，如果使用 后续的中间件和handler就直接跳过了
	c.JSON(code, H{"message": err})
}

func (c *Context) Param(key string) string {
	//获取路由解析后的参数对应的值
	value, _ := c.Params[key]
	return value
}

func (c *Context) PostForm(key string) string {
	//获取post请求中?后面的参数：http://localhost:9999/login?username=geektutu&password=1234
	return c.Req.FormValue(key)
}

func (c *Context) Query(key string) string {
	return c.Req.URL.Query().Get(key)
}

func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Writer.WriteHeader(code)
}

func (c *Context) SetHeader(key string, value string) {
	c.Writer.Header().Set(key, value)
}

func (c *Context) String(code int, format string, values ...interface{}) {
	c.SetHeader("Content-Type", "text/plain")
	c.Status(code)
	c.Writer.Write([]byte(fmt.Sprintf(format, values...)))
}

func (c *Context) JSON(code int, obj interface{}) {
	c.SetHeader("Content-Type", "application/json")
	c.Status(code)
	encoder := json.NewEncoder(c.Writer) //NewEncoder创建一个将数据写入w的Encoder
	if err := encoder.Encode(obj); err != nil { //Encode将v的json编码写入输出流，并会写入一个换行符
		http.Error(c.Writer, err.Error(), 500)
	}
}

func (c *Context) Data(code int, data []byte) {
	c.Status(code)
	c.Writer.Write(data)
}

func (c *Context) HTML(code int, name string, data interface{}) {
	c.SetHeader("Content-Type", "text/html")
	c.Status(code)
	//模板渲染
	if err := c.engine.htmlTemplates.ExecuteTemplate(c.Writer, name, data); err != nil {
		c.Fail(500, err.Error())
	}
}

//在使用Context.ResponseWriter中的Set/WriteHeader/Write这三个方法时，使用顺序必须如下所示，否则会出现某一设置不生效的情况。
//ctx.ResponseWriter.Header().Set("Content-type", "application/text")
//ctx.ResponseWriter.WriteHeader(403)
//ctx.ResponseWriter.Write([]byte(resp))