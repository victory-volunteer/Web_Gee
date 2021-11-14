package gee

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
)

// 获取调用栈信息
//获取触发 panic 的堆栈信息
func trace(message string) string {
	var pcs [32]uintptr //调用栈标识符
	//Callers 用来返回调用栈的程序计数器, 第 0 个 Caller 是 Callers 本身，第 1 个是上一层 trace，第 2 个是再上一层的 defer func。
	//因此，为了日志简洁一点，我们跳过了前 3 个 Caller
	n := runtime.Callers(3, pcs[:]) // 跳过前3个caller(n为返回写入到pcs中的项数)
	var str strings.Builder
	str.WriteString(message + "\nTraceback:")
	for _, pc := range pcs[:n] {
		fn := runtime.FuncForPC(pc) //获取对应的函数
		file, line := fn.FileLine(pc) //获取到调用栈所调用的函数的源代码文件名和行号，打印在日志中
		str.WriteString(fmt.Sprintf("\n\t%s:%d", file, line))
	}
	return str.String() //是一个空行
}

//net/http 的源码中，也使用了 recovery ，所以一般不会导致web服务崩溃，即其他请求不受影响
// Recovery 中间件在这里的作用还是保证所有请求都能正常响应，否则，panic 之后，就没回应了
func Recovery() HandlerFunc {
	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil {
				message := fmt.Sprintf("%s", err)
				fmt.Println("报错开始")
				log.Printf("Recovery:%s\n\n", trace(message))
				fmt.Println("报错结束")
				c.Fail(http.StatusInternalServerError, "Internal Server Error")
			}
		}()
		c.Next()
	}
}
