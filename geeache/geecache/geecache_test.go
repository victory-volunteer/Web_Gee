package geecache

import (
	"fmt"
	"log"
	"reflect"
	"testing"
)

//模拟耗时的数据库
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func TestGetter(t *testing.T) {
	//声明一个Getter类型的变量f,借助 GetterFunc 的类型转换，将一个匿名回调函数转换成了接口 f Getter
	var f Getter = GetterFunc(func(key string) ([]byte, error) {
		return []byte(key), nil
	})

	expect := []byte("key")
	//调用该接口的方法 f.Get(key string)，实际上就是在调用匿名回调函数
	if v, _ := f.Get("key"); !reflect.DeepEqual(v, expect) {
		t.Fatal("回调 failed")
	}
}

//1）在缓存为空的情况下，能够通过回调函数获取到源数据(缓存为空时，调用了回调函数，第二次访问时，则直接从缓存中读取)
//2）在缓存已经存在的情况下，是否直接从缓存中获取
//为了实现这一点，使用 loadCounts 统计某个键调用回调函数的次数，如果次数大于1，则表示调用了多次回调函数，没有缓存
func TestGet(t *testing.T) {
	loadCounts := make(map[string]int, len(db))
	gee := NewGroup("scores", 2<<10, GetterFunc( //2<<10表示将2的二进制位左移10位
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				if _, ok := loadCounts[key]; !ok {
					loadCounts[key] = 0
				}
				loadCounts[key]++
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	for k, v := range db {
		if view, err := gee.Get(k); err != nil || view.String() != v { //缓存为空时，调用了回调函数
			t.Fatal("failed to get value of Tom")
		}
		if _, err := gee.Get(k); err != nil || loadCounts[k] > 1 {  //第二次访问时，则直接从缓存中读取
			t.Fatalf("cache %s miss", k)
		}
	}

	if view, err := gee.Get("unknown"); err == nil { //上面db中找不到key=unknown
		t.Fatalf("unknow的值应该是空的，但是%s被获取了", view)
	}

}

func TestGetGroup(t *testing.T) {
	groupName := "scores"
	NewGroup(groupName, 2<<10, GetterFunc(
		func(key string) (bytes []byte, err error) { return }))

	if group := GetGroup(groupName); group == nil || group.name != groupName {
		t.Fatalf("group %s not exist", groupName)
	}

	if group := GetGroup(groupName + "111"); group != nil {
		t.Fatalf("expect nil, but %s got", group.name)
	}
}