package geecache
/**
分布式缓存需要实现节点间通信，建立基于 HTTP 的通信机制是比较常见和简单的做法
提供被其他节点访问的能力(基于http)
*/
import (
	"fmt"
	"geecache/consistenthash"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	pb "geecache/geecachepb"

	"github.com/golang/protobuf/proto"
)

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50 //虚拟节点倍数
)

//结构体 HTTPPool，作为承载节点间 HTTP 通信的核心数据结构(包括服务端和客户端)
type HTTPPool struct {
	// this peer's base URL, e.g. "https://example.net:8000"
	self        string                 //记录自己的地址，包括主机名/IP 和端口
	basePath    string                 //作为节点间通讯地址的前缀，默认是 /_geecache/
	mu          sync.Mutex             // guards peers and httpGetters
	peers       *consistenthash.Map    //类型是一致性哈希算法的 Map，用来根据具体的 key 选择节点
	//映射远程节点与对应的 httpGetter。每一个远程节点对应一个 httpGetter，因为 httpGetter 与远程节点的地址 baseURL 有关
	httpGetters map[string]*httpGetter //key_eg: "http://10.0.0.2:8008"
}

// NewHTTPPool initializes an HTTP pool of peers.
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log info with server name
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[当前服务:%s] %s", p.self, fmt.Sprintf(format, v...))
}

// 通过路由获取groupname(需要提前创建缓存组，若没有在groups中找到会报错)和
//key(在对应的缓存组中寻找key对应的缓存值，没有找到则会调用回调函数创建并添加至缓存组)，最终打印key对应的缓存值
//请求路由示例：http://localhost:8001/_geecache/scores/Tom
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//处理http路由
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("响应方法:%s 路由:%s", r.Method, r.URL.Path)
	// 将/<basepath>/<groupname>/<key>拆分为["<groupname>","<key>"]
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key) //获取组中key对应的缓存
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println("获取缓存值失败")
		return
	}

	// 使用 proto.Marshal() 编码 HTTP 响应
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	//w.Write(view.ByteSlice()) //将缓存值作为 httpResponse 的 body 返回
	w.Write(body)
}

//注册传入的peers节点，并为每一个节点创建节点间通讯地址
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil) // 实例化一致性哈希算法
	p.peers.Add(peers...) //添加了传入的节点
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers { //为每一个节点创建了一个 HTTP 客户端 httpGetter
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
		fmt.Printf("HTTPPool.httpGetters中 节点:%v--通讯地址:%v\n",peer,peer + p.basePath)
	}

}

// 根据具体的 key，得到应该存放的真实节点，返回真实节点对应的 httpGetter (HTTP 客户端)
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("真实节点%s", peer)
		fmt.Printf("%v存放在真实节点%v上",key,peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

//HTTP 客户端类
type httpGetter struct {
	baseURL string //将要访问的远程节点的地址，例如 http://example.com/_geecache/
}

//返回客户端响应中body(即group中key对应的缓存值)
//使用 http.Get() 方式获取返回值，并转换为 []bytes 类型
func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
//func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		//url.QueryEscape(group), //对字符串进行网址编码转译
		//url.QueryEscape(key),
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)
	fmt.Printf("客户端Get方法中url:%v\n",u)
	res, err := http.Get(u) //这里直接到了ServeHTTP,这个流程中又走了一遍(g *Group) load(key string) (value ByteView, err error)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	//使用 proto.Unmarshal() 解码 HTTP 响应,传入out中,作为(g *Group) getFromPeer方法中的res.Value
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}

	//return bytes, nil
	return nil
}

//确保这个类型实现了这个接口 如果没有实现会报错的
var _ PeerGetter = (*httpGetter)(nil)
