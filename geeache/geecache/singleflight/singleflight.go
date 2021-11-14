package singleflight
/**
解决问题：
并发了 N 个请求 ?key=Tom，8003 节点向 8001 同时发起了 N 次请求。
假设对数据库的访问没有做任何限制的，很可能向数据库也发起 N 次请求，容易导致缓存击穿和穿透

当一瞬间有大量请求get(key)，而且key未被缓存或者未被缓存在当前节点 如果不用singleflight，
那么这些请求都会发送远端节点或者从本地数据库读取，会造成远端节点或本地数据库压力猛增。使用singleflight，
第一个get(key)请求到来时，singleflight会记录当前key正在被处理，后续的请求只需要等待第一个请求处理完成，取返回值即可
*/
import "sync"

// 代表正在进行中，或已经结束的请求。使用 sync.WaitGroup 锁避免重入
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// Group 是 singleflight 的主数据结构，管理不同 key 的请求(call)
type Group struct {
	mu sync.Mutex //保护 Group 的成员变量 m 不被并发读写而加上的锁
	m  map[string]*call
}

//针对相同的 key，无论 Do 被调用多少次，函数 fn 都只会被调用一次，等待 fn 调用结束了，返回返回值或错误
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock() //保护下面的g.m[key]
	if g.m == nil {
		g.m = make(map[string]*call) // 延迟初始化
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait() //阻塞当前的gorotinue
		// 当其他访问者获取到c，说明key已经有对应的请求正在进行中，则等待原始调用完成后直接复用这个结果
		return c.val, c.err
	}

	c := new(call)
	c.wg.Add(1)
	g.m[key] = c //添加到 g.m，表明 key 已经有对应的请求正在在处理
	g.mu.Unlock()
	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key) //若不删除，如果key对应的值变化，所得的值还是旧值,且占用内存
	//因为singleflight机制相当于是一个请求的缓冲器，不需要有储存功能
	//在少量访问时，正常使用
	//在大量并发访问时，对于并发的信息，共享第一个请求的返回值，大幅减少请求次数
	g.mu.Unlock()

	return c.val, c.err
}
