package geecache
/**
缓存值的抽象与封装
*/

//抽象了一个只读数据结构 ByteView 用来表示缓存值
type ByteView struct {
	//b 将会存储真实的缓存值。选择 byte 类型是为了能够支持任意的数据类型的存储，例如字符串、图片等
	b []byte
}

//返回被缓存对象所占的内存大小
func (v ByteView) Len() int {
	return len(v.b)
}

// b 是只读的，ByteSlice()以字节片的形式返回数据的副本拷贝,防止缓存值被外部程序修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// String以字符串形式返回数据，如有必要则进行复制
func (v ByteView) String() string {
	return string(v.b)
}

//b是切片，切片不会深拷贝
func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b) //若输入字符串给b,则当成是[]byte类型切片
	return c
}
