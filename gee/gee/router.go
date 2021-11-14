package gee

import (
	"net/http"
	"strings"
)

type router struct {
	roots    map[string]*node //存储每种请求方式的Trie 树根节点
	handlers map[string]HandlerFunc //存储每种请求方式的 HandlerFunc
}

func newRouter() *router {
	return &router{
		roots:    make(map[string]*node),
		handlers: make(map[string]HandlerFunc),
	}
}

//传入/p/*name/*，传出{"p", "*name"}
func parsePattern(pattern string) []string {
	vs := strings.Split(pattern, "/")
	parts := make([]string, 0)
	for _, item := range vs {
		if item != "" {
			parts = append(parts, item)
			if item[0] == '*' {
				break
			}
		}
	}
	return parts
}

//给结构体增加路由节点
func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
	parts := parsePattern(pattern)

	key := method + "-" + pattern
	_, ok := r.roots[method]
	if !ok {
		r.roots[method] = &node{} //为了使每个roots[method]都成为*node对象，方便调用insert()
	}
	r.roots[method].insert(pattern, parts, 0)
	r.handlers[key] = handler
}

//解析了:和*两种匹配符的参数，返回一个 map
//例如/p/go/doc匹配到/p/:lang/doc，解析结果为：{lang: "go"}，
// /static/css/geektutu.css匹配到/static/*filepath，解析结果为{filepath: "css/geektutu.css"}
func (r *router) getRoute(method string, path string) (*node, map[string]string) {
	searchParts := parsePattern(path)
	params := make(map[string]string)
	root, ok := r.roots[method]

	if !ok {
		return nil, nil
	}
	n := root.search(searchParts, 0) //成功匹配的路由节点
	if n != nil {
		parts := parsePattern(n.pattern) //n.pattern拿到成功匹配到的节点上面的完整路径
		for index, part := range parts {
			if part[0] == ':' {
				params[part[1:]] = searchParts[index]
			}
			if part[0] == '*' && len(part) > 1 {
				//使len(part) > 1才有可以被赋值的对象
				params[part[1:]] = strings.Join(searchParts[index:], "/") //将切片以'/'为分隔符组合成一个string
				break
			}
		}
		return n, params
	}

	return nil, nil
}

func (r *router) getRoutes(method string) []*node {
	root, ok := r.roots[method] //获取method方法的第一个节点
	if !ok {
		return nil
	}
	nodes := make([]*node, 0)
	root.travel(&nodes) //获取method方法下的所有已保存节点完整路径(路由)
	return nodes
}

func (r *router) handle(c *Context) {
	n, params := r.getRoute(c.Method, c.Path)

	if n != nil {
		key := c.Method + "-" + n.pattern
		//在调用匹配到的handler前，将解析出来的路由参数赋值给了c.Params
		c.Params = params
		c.handlers = append(c.handlers, r.handlers[key]) //将从路由匹配得到的 Handler 添加到 c.handlers列表中，执行c.Next()
	} else {
		c.handlers = append(c.handlers, func(c *Context) {
			c.String(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
		})
	}
	c.Next()
}
