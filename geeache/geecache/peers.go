package geecache

import pb "geecache/geecachepb"

type PeerPicker interface {
	// 根据具体的 key，得到应该存放的真实节点，返回真实节点对应的 httpGetter (HTTP 客户端)
	PickPeer(key string) (peer PeerGetter, ok bool)
}

//PeerGetter 就对应于上述流程中的 httpGetter (HTTP 客户端)
//type PeerGetter interface {
//	//返回客户端响应中body(即group中key对应的缓存值)
//	Get(group string, key string) ([]byte, error)
//}

//使用protobuf修改，参数使用 geecachepb.pb.go 中的数据类型
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}