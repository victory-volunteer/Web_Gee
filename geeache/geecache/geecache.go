package geecache
/**
负责与外部交互，控制缓存存储和获取的主流程
*/
import (
	"fmt"
	"geecache/singleflight"
	"log"
	"sync"
	pb "geecache/geecachepb"
)

/**
Group 是 GeeCache 最核心的数据结构,负责与用户的交互，并且控制缓存值存储和获取的流程
接收 key --> 检查是否被缓存 --是--> 返回缓存值⑴
               |
               |--否--> 是否应当从远程节点获取 --是--> 与远程节点交互 --> 返回缓存值⑵
                           |
                           |--否--> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值⑶
细化流程 ⑵：
使用一致性哈希选择节点        是                                    是
    |-----> 是否是远程节点 -----> HTTP 客户端访问远程节点 --> 成功？-----> 服务端返回返回值
                    |  否                                    ↓  否
                    |----------------------------> 回退到本地节点处理。
*/

//一个 Group 可以认为是一个缓存的命名空间，每个 Group 拥有一个唯一的名称 name。
//比如可以创建三个 Group，缓存学生的成绩命名为 scores，缓存学生信息的命名为 info，缓存学生课程的命名为 courses。
type Group struct {
	name      string
	getter    Getter //缓存未命中时获取源数据的回调(callback)，接口作为参数，便于扩展（接口内新增方法）
	mainCache cache  //一开始实现的并发缓存(分布式中本地分配到的cache部分)
	peers     PeerPicker //储存实现了 PeerPicker 接口的 HTTPPool
	loader *singleflight.Group //确保每个key只被获取一次
}

type Getter interface {
	Get(key string) ([]byte, error)
}

//函数类型实现某一个接口，称之为接口型函数(接口型函数只能应用于接口内部只定义了一个方法的情况)
//方便使用者在调用时既能够传入函数作为参数,也能够传入实现了该接口的结构体作为参数
//定义一个函数类型 F，并且实现接口 A 的方法，然后在这个方法中调用自己。
//这是 Go 语言中将其他函数（参数返回值定义与 F 一致）转换为接口 A 的常用技巧。
type GetterFunc func(key string) ([]byte, error)

// 回调函数，在缓存不存在时，调用这个函数，得到源数据(由用户决定如何从源头获取数据)
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key) //调用自己
}

var (
	mu     sync.RWMutex //读写互斥锁
	groups = make(map[string]*Group)
) //初始化全局变量

//实例化 Group，并且将 group 存储在全局变量 groups 中
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock() //加写锁
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

//GetGroup返回先前用NewGroup创建的命名组，如果没有这样的组，则返回nil。
//用来特定名称的 Group，这里使用了只读锁 RLock()，因为不涉及任何冲突变量的写操作
func GetGroup(name string) *Group {
	mu.RLock() //加读锁
	g := groups[name]
	mu.RUnlock()
	return g
}

// 从缓存中获取键的值
//Get 方法实现了上述所说的流程 ⑴ 和 ⑶。
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok { //从 mainCache 中查找缓存，如果存在则返回缓存值
		log.Printf("从mainCache中查找到%v对应缓存:%v\n",key,v)
		return v, nil
	}

	return g.load(key) //缓存不存在，则调用 load 方法创建
}

// 将实现了 PeerPicker 接口的 HTTPPool 注入到 Group 中
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

//（分布式场景下，load 会先从远程节点获取 getFromPeer，失败了再回退到 getLocally 本地处理，设计时预留了）
//使用 PickPeer() 方法选择节点，若非本机节点，则调用 getFromPeer() 从远程获取。若是本机节点或失败，则回退到 getLocally()
func (g *Group) load(key string) (value ByteView, err error) {
	//每个key只被获取一次(本地或远程),不考虑并发调用者的数量
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			fmt.Printf("g.peers存有HTTPPool:%v\n",g.peers)
			if peer, ok := g.peers.PickPeer(key); ok { //获取一个客户端对象
				fmt.Printf("真实节点对应客户端对象(通讯地址):%v\n",peer)
				if value, err = g.getFromPeer(peer, key); err == nil { //此处从远端获取缓存后，并没有储存到 mainCache 缓存
					fmt.Printf("使用客户端访问远程节点获取到缓存值:%v\n",value)
					return value, nil
				}
				log.Println("客户端获取失败", err)
			}
		}
		fmt.Println("开始从本地回调函数获取缓存值")
		return g.getLocally(key)
	})
	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

//将键值对存储到 mainCache 缓存中，然后将更新后的值返回给调用者
func (g *Group) populateCache(key string, value ByteView) {
	fmt.Println("将本地数据源存入缓存")
	g.mainCache.add(key, value)
}

//调用用户注册的回调函数回填缓存
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key) //调用用户回调函数(可能是从数据库中加载数据)获取源数据，创建一条缓存值记录
	fmt.Printf("得到[]byte处理后的本地数据源:%v err: %v\n",bytes,err)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)} //bytes是切片，切片不会深拷贝
	g.populateCache(key, value)
	return value, nil
}

//使用实现了 PeerGetter 接口的 httpGetter 访问远程节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	//bytes, err := peer.Get(g.name, key)
	//fmt.Printf("访问客户端Get方法中url返回:%v\n",bytes)
	//if err != nil {
	//	return ByteView{}, err
	//}
	//return ByteView{b: bytes}, nil

	//使用protobuf修改
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	//原来的bytes由res.Value( 由(h *httpGetter) Get方法中中生成 )提供
	return ByteView{b: res.Value}, nil
}
