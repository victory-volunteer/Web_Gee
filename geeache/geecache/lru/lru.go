package lru
/**
lru 缓存淘汰策略
*/
import "container/list"

// Cache是一个LRU缓存。它对于并发访问不安全。
type Cache struct {
	maxBytes int64 //允许使用的最大内存
	nbytes   int64 //当前已使用的内存
	ll       *list.List //双向链表(存放entry结构体)
	cache    map[string]*list.Element //值是双向链表中对应节点的指针
	OnEvicted func(key string, value Value) //某条记录被移除时的回调函数，可以为 nil
}

type entry struct {
	//双向链表节点的数据类型，在链表中仍保存每个值对应的 key 的好处在于，
	//淘汰队首节点时，需要用 key 从字典中删除对应的映射
	key   string
	value Value //缓存值
}

type Value interface {
	Len() int //用于返回值所占用的内存大小
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes, //若传入int64(0)，则这里假定可以无限添加
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// Add向缓存中添加一个值
func (c *Cache) Add(key string, value Value) {
	//ele是链表节点的指针，*ele就是节点
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele) //将该节点移到队尾
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len()) //新值减去老值的长度
		kv.value = value //如果键存在，则更新原节点的值
	} else {
		//队尾添加新节点 &entry{key, value}, 并字典中添加 key 和节点的映射关系
		ele := c.ll.PushFront(&entry{key, value})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	//更新 c.nbytes，如果超过了设定的最大值 c.maxBytes，则移除最少访问的节点
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		// 使用for，因为当添加一条大的键值对时，c.nbytes可能会变得很大，
		//可能需要淘汰掉多个键值对，直到 c.maxBytes < c.nbytes
		c.RemoveOldest()
	}
}

// Get查找一个键的值
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		//如果键对应的链表节点存在，则将对应节点移动到队尾，并返回查找到的值
		//将链表中的节点 ele 移动到队尾（双向链表作为队列，队首队尾是相对的，在这里约定 front 为队尾）
		c.ll.MoveToFront(ele)
		//Element这个结构体的源码，value字段存的值是interface{}类型的，ele.Value是一个空接口类型
		//该语法返回两个参数，第一个参数是Value转化为*entry类型后的变量，第二个值是一个布尔值
		kv := ele.Value.(*entry)
		//kv.value是一个接口类型
		return kv.value, true
	}
	return
}

// 缓存淘汰。即移除最近最少访问的节点（队首）
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back() //取到队首节点，从链表中删除
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key) //从字典中 c.cache 删除该节点的映射关系
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len()) //更新当前所用的内存
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) Len() int {
	return c.ll.Len() //返回链表中元素的个数
}
