package gee

import (
	"fmt"
	"strings"
)

type node struct {
	pattern  string  // 完整路由，例如 /p/:lang
	part     string  // 路由中的一部分(当前节点)，例如 :lang
	children []*node // 子节点，例如 [doc, tutorial, intro]
	isWild   bool    // 是否模糊匹配，part 含有 : 或 * 时为true，默认为False
}

func (n *node) String() string {
	return fmt.Sprintf("node{pattern=%s, part=%s, isWild=%t}", n.pattern, n.part, n.isWild)
}

//插入功能：递归查找每一层的节点，如果没有匹配到当前part的节点，则新建一个。
//有一点需要注意，/p/:lang/doc只有在第三层节点，即doc节点，pattern才会设置为/p/:lang/doc，p和:lang节点的pattern属性皆为空。
//因此，当匹配结束时，我们可以使用n.pattern == ""来判断路由规则是否匹配成功。
//例如，/p/python虽能成功匹配到:lang，但:lang的pattern值为空，因此匹配失败
//传入/p/*name/*,{"p", "*name"},0
func (n *node) insert(pattern string, parts []string, height int) {
	if len(parts) == height {
		// 如果已经匹配完了，那么将pattern赋值给该node，表示它是一个完整的url
		n.pattern = pattern
		return
	}

	part := parts[height]
	child := n.matchChild(part) //查看当前路由节点是否在已有的路由节点子列表里
	if child == nil {
		child = &node{part: part, isWild: part[0] == ':' || part[0] == '*'}
		n.children = append(n.children, child)
	}
	child.insert(pattern, parts, height+1)
}

//查询功能，同样也是递归查询每一层的节点，退出规则是，匹配到了*，匹配失败，或者匹配到了第len(parts)层节点。
func (n *node) search(parts []string, height int) *node {
	if len(parts) == height || strings.HasPrefix(n.part, "*") {
		//len(parts) == height代表用户通过r.getRoute()输入的路径已全部匹配完
		//strings.HasPrefix用来检测当前已保存节点是否以指定的前缀开头
		if n.pattern == "" {
			return nil
		}
		return n
	}

	part := parts[height]
	children := n.matchChildren(part) // 获取所有可能的子路径

	for _, child := range children {
		//把成功匹配的所有二级路径再到它们的三级路径去匹配，以此类推
		result := child.search(parts, height+1)
		if result != nil {
			return result
		}
	}

	return nil
}

// 查找所有已经注册的完整路由，保存到列表中
func (n *node) travel(list *([]*node)) {
	if n.pattern != "" {
		*list = append(*list, n)
	}
	for _, child := range n.children {
		child.travel(list)
	}
}

// 找到匹配的子节点，场景是用在插入时使用，找到1个匹配的就立即返回(查询节点在已有节点子列表中是否存在，存在则匹配成功)
func (n *node) matchChild(part string) *node {
	for _, child := range n.children {
		if child.part == part || child.isWild {
			return child
		}
	}
	return nil
}

// 所有匹配成功的节点，用于查找
// 它必须返回所有可能的子节点来进行遍历查找
func (n *node) matchChildren(part string) []*node {
	nodes := make([]*node, 0)
	for _, child := range n.children {
		if child.part == part || child.isWild {
			nodes = append(nodes, child)
		}
	}
	return nodes
}
