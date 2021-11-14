package consistenthash

import (
	"fmt"
	"strconv"
	"testing"
)

//自定义的 Hash 算法。自定义的 Hash 算法只处理数字，传入字符串表示的数字，返回对应的数字即可。
//一开始，有 2/4/6 三个真实节点，对应的虚拟节点的哈希值是 02/12/22、04/14/24、06/16/26。
//那么用例 2/11/23/27 选择的虚拟节点分别是 02/12/24/02，也就是真实节点 2/2/4/2。
//添加一个真实节点 8，对应虚拟节点的哈希值是 08/18/28，此时，用例 27 对应的虚拟节点从 02 变更为 28，即真实节点 8。
func TestHashing(t *testing.T) {
	hash := New(3, func(key []byte) uint32 {
		i, _ := strconv.Atoi(string(key)) //将字符串类型的整数转换为int类型
		return uint32(i)
	})

	// Given the above hash function, this will give replicas with "hashes":
	// 2, 4, 6, 12, 14, 16, 22, 24, 26
	hash.Add("6", "4", "2")

	testCases := map[string]string{
		"2":  "2",
		"11": "2",
		"23": "4",
		"27": "2",
	} //key对应的用例，value对应的真实节点

	for k, v := range testCases {
		if hash.Get(k) != v {
			t.Errorf("请求%s, 应该得到%s", k, v)
		}
	}
	fmt.Println("-----------")
	// Adds 8, 18, 28
	hash.Add("8")

	// 27 should now map to 8.
	testCases["27"] = "8"

	for k, v := range testCases {
		if hash.Get(k) != v {
			t.Errorf("Asking for %s, should have yielded %s", k, v)
		}
	}

}
