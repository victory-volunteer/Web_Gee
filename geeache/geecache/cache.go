package geecache
/**
并发控制（实例化 lru，封装 get 和 add 方法，并添加互斥锁 mu）
*/
import (
	"geecache/lru"
	"sync"
)

type cache struct {
	mu         sync.Mutex //cache 的 get 和 add 都涉及到写操作(LRU 将最近访问元素移动到链表头)，所以不能直接改为读写锁
	lru        *lru.Cache
	cacheBytes int64 //允许使用的最大内存
}

//判断了 c.lru 是否为 nil，如果等于 nil 再创建实例。
//这种方法称之为延迟初始化(Lazy Initialization)，
//一个对象的延迟初始化意味着该对象的创建将会延迟至第一次使用该对象时。主要用于提高性能，并减少程序内存要求
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok //转换为ByteView类型
	}

	return
}
