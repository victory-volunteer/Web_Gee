package gee

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
)

//提供给框架用户，用来定义路由映射的处理方法
type HandlerFunc func(*Context)

// Engine实现ServeHTTP接口
type (
	RouterGroup struct {
		prefix      string        // 前缀(初始为空字符串)
		middlewares []HandlerFunc // 支持中间件
		parent      *RouterGroup  // 支持分组嵌套（保存父组对象） (暂时没有用到，可以删除)
		engine      *Engine       // 所有组共享一个Engine实例（只在初始化时赋值一次，以后所有组都使用初始赋值，目的在于继承）
		//为了Group有访问Router的能力，在Group中保存一个指针，指向Engine，整个框架的所有资源都是由Engine统一协调的，那么就可以通过Engine间接地访问各种接口了
	}

	Engine struct {
		*RouterGroup
		//Go语言的嵌套在其他语言中类似于继承，子类必然是比父类有更多的成员变量和方法
		//RouterGroup 仅仅是负责分组路由，Engine 除了分组路由外，还有很多其他的功能
		//将Engine作为最顶层的分组，也就是说Engine拥有RouterGroup所有的能力
		//通过结构体嵌套实现继承（只在初始化时赋值一次，以后所有组都使用初始赋值，目的在于继承）
		router        *router
		groups        []*RouterGroup // 存储所有组
		htmlTemplates *template.Template // 将所有的模板加载进内存（gee框架的模板渲染直接使用html/template提供的能力）
		funcMap       template.FuncMap   // 所有的自定义模板渲染函数
	}
)

// New是gee.Engine的构造函数
func New() *Engine {
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}

func Default() *Engine {
	engine := New()
	engine.Use(Logger(), Recovery())
	return engine
}

//定义Group来创建一个新的RouterGroup(该变量内的engine结构实际上等于上一个group内的engine)
//记住所有组共享同一个Engine实例
func (group *RouterGroup) Group(prefix string) *RouterGroup {
	engine := group.engine
	newGroup := &RouterGroup{
		prefix: group.prefix + prefix,
		parent: group, //父组
		engine: engine,
	}
	engine.groups = append(engine.groups, newGroup)
	return newGroup
}

// Use定义为添加中间件到组
func (group *RouterGroup) Use(middlewares ...HandlerFunc) {
	group.middlewares = append(group.middlewares, middlewares...)
}

func (group *RouterGroup) addRoute(method string, comp string, handler HandlerFunc) {
	pattern := group.prefix + comp //分组之前初始路由是"/"，分组之后初始路由是原来的节点前缀+当前字符comp
	log.Printf("Route %4s - %s", method, pattern)
	group.engine.router.addRoute(method, pattern, handler)
}

func (group *RouterGroup) GET(pattern string, handler HandlerFunc) {
	group.addRoute("GET", pattern, handler)
}

func (group *RouterGroup) POST(pattern string, handler HandlerFunc) {
	group.addRoute("POST", pattern, handler)
}

//加载静态文件
func (group *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
	//拼接父组的路由前缀和当前路由
	absolutePath := path.Join(group.prefix, relativePath)
	//gee框架要做的，仅仅是解析请求的地址，映射到服务器上文件的真实地址，交给http.FileServer处理就好了
	//StripPrefix用于过滤掉url中的absolutePath前缀，留下静态文件存放路径（https://blog.csdn.net/a13602955218/article/details/106692668/）
	//fs是存放静态文件的磁盘文件路径也就是Static()里传的root
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))
	return func(c *Context) {
		file := c.Param("filepath")
		if _, err := fs.Open(file); err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		fileServer.ServeHTTP(c.Writer, c.Req) //交由http去处理
	}
}

// 单独做静态文件访问
//Static这个方法是暴露给用户的。用户可以将磁盘上的某个文件夹root映射到路由relativePath
func (group *RouterGroup) Static(relativePath string, root string) {
	//FileServer接收一个FileSystem类型参数，FileSystem是一个接口，http.Dir这个类型实现了该接口
	handler := group.createStaticHandler(relativePath, http.Dir(root))
	//注册路由/relativePath/*filepath,用户访问localhost:9999/relativePath/js/geektutu.js
	urlPattern := path.Join(relativePath, "/*filepath")
	//path.Join()使用特定分隔符作为定界符将所有给定的path片段连接在一起,此处urlPattern用来注册路由
	group.GET(urlPattern, handler)
}

// 自定义模板函数
func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

//模板解析
func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
}

func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine)
}

//第一个参数是 ResponseWriter ，利用 ResponseWriter 可以构造针对该请求的响应
//第二个参数是 Request ，该对象包含了该HTTP请求的所有的信息，比如请求地址、Header和Body等信息；
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var middlewares []HandlerFunc
	for _, group := range engine.groups {
		//接收到一个具体请求时，通过 URL 的前缀判断该请求适用于哪些中间件
		if strings.HasPrefix(req.URL.Path, group.prefix) {
			middlewares = append(middlewares, group.middlewares...)
		}
	}
	c := newContext(w, req) //在调用router.handle之前，构造一个 Context 对象
	c.handlers = middlewares //注册中间件其实就是将中间件函数追加到handlers中
	c.engine = engine
	engine.router.handle(c)
}
