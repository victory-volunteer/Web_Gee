package consistenthash

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
)

//采取依赖注入的方式，允许用于替换成自定义的 Hash 函数，也方便测试时替换，默认为 crc32.ChecksumIEEE 算法
type Hash func(data []byte) uint32

// Map constains all hashed keys
type Map struct {
	hash     Hash //自定义Hash函数
	replicas int //虚拟节点倍数
	keys     []int // 哈希环
	hashMap  map[int]string //虚拟节点与真实节点的映射表，键是虚拟节点的哈希值，值是真实节点的名称
}

//  允许自定义虚拟节点倍数和 Hash 函数
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// 生成虚拟节点经过哈希处理后添加至环上并排序
// 添加真实节点/机器（允许传入 0 或 多个真实节点的名称）
func (m *Map) Add(keys ...string) {
	count:=0
	for _, key := range keys {
		//对每一个真实节点 key，对应创建 m.replicas 个虚拟节点
		for i := 0; i < m.replicas; i++ {
			//虚拟节点的名称是：(将数字i转为字符串)strconv.Itoa(i) + key，即通过添加编号的方式区分不同虚拟节点
			//使用 m.hash() 计算虚拟节点的哈希值，使用 append(m.keys, hash) 添加到环上
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash) //将虚拟节点的哈希值添加到环上
			m.hashMap[hash] = key //在 hashMap 中增加虚拟节点和真实节点的映射关系
			count+=1
		}
	}
	sort.Ints(m.keys) //环上的虚拟节点(哈希值)升序排列
	fmt.Printf("虚拟节点个数:%v\n",count)
}

//使用key经过哈希处理，最终返回key应该存放在哪个真实节点上(在哈希环上顺时针查找)
func (m *Map) Get(key string) string {
	//第一步，计算 key 的哈希值。
	//第二步，顺时针找到第一个匹配的虚拟节点的下标 idx，从 m.keys 中获取到对应的哈希值。
	//如果 idx == len(m.keys)即key比m.keys都大的时候，说明应选择 m.keys[0]（大于最大值时顺时针查找节点索引到 0 号节点）
	//因为 m.keys 是一个环状结构，所以用取余数的方式(idx%len(m.keys)会得到0)来处理这种情况。
	//第三步，通过 hashMap 映射得到真实的节点。

	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	//fmt.Printf("Get-hash:%v\n",hash)
	// 二分法在环中查找满足条件的相应值，结果为false向后查，反之向前查，结果返回满足条件的第一个索引；若查询不到，则返回len(m.keys)
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	//fmt.Printf("Get-idx:%v 虚拟节点索引:%v 虚拟节点:%v\n",idx,idx%len(m.keys),m.keys[idx%len(m.keys)])
	return m.hashMap[m.keys[idx%len(m.keys)]]
}

// 删除环和映射上的节点及虚拟节点（自添加）
func (m *Map) Remove(key string) {
	for i := 0; i < m.replicas; i++ {
		hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
		idx := sort.SearchInts(m.keys, hash)
		m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
		delete(m.hashMap, hash)
	}
}