package main

/*
过程分析：http://localhost:8001/_geecache/scores/Tom
目的：查找key(Tom)对应的缓存，url中的group(scores)需要提前创建好
1.从本地mainCache中查找缓存，没有则从远程节点获取
2.根据具体的key，得到应该存放在哪个真实节点(同样的key对应的哈希是相同的，所有选择的真实节点始终不变)，返回真实节点对应的httpGetter(HTTP 客户端对象)
3.由httpGetter(HTTP 客户端对象)的Get方法经过url整合获取响应body(在这里作为key对应的缓存值)
4.第3步失败，则调用用户注册的回调函数获取数据源中key对应的值作为缓存值，并存入本地mainCache中
5.下次在访问相同的节点时，直接从本地mainCache中查找得到缓存

结果分析：传入example -port=8001 & example -port=8002 & example -port=8003 -api=1 来启动3个服务端
                 当并发了 3 个请求 ?key=Tom，从日志中可以看到，三次均选择了节点 8001，这是一致性哈希算法的功劳。
	这仅仅同时向 8001 发起了 3 次请求，假如有 10 万个在并发请求该数据，那就会向 8001 同时发起 10 万次请求，如果 8001 又同时向数据库发起 10 万次查询请求，很容易导致缓存被击穿。
	此时需要给请求加锁，当其他进程想要访问该请求时会阻塞，等待第一个访问这个请求的进程返回时一起返回响应。
*/

import (
	"flag"
	"fmt"
	"geecache"
	"log"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
} //使用 map 模拟数据源

func createGroup() *geecache.Group {
	return geecache.NewGroup("scores", 2<<10, geecache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("在数据源中寻找", key)
			if v, ok := db[key]; ok {
				fmt.Println(v)
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

//启动缓存服务器：创建 HTTPPool，添加节点信息，注册到 gee 中，启动 HTTP 服务（共3个端口，8001/8002/8003）
//addrs=[http://localhost:8001 http://localhost:8002 http://localhost:8003]

func startCacheServer(addr string, addrs []string, gee *geecache.Group) {
	peers := geecache.NewHTTPPool(addr)
	peers.Set(addrs...)  //注册虚拟节点并且注入节点和通讯地址的对应关系
	gee.RegisterPeers(peers)
	log.Printf("geecache is running at %v--%v\n", addr,addr[7:])
	log.Fatal(http.ListenAndServe(addr[7:], peers)) //会进入(p *HTTPPool) ServeHTTP
}

func main() {
	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "Geecache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	//命令行传入 port 和 api 2 个参数，用来在指定端口启动 HTTP 服务
	flag.Parse()

	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	gee := createGroup()

	fmt.Println(addrMap[port])
	startCacheServer(addrMap[port], addrs, gee)
}
