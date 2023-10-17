package tests

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const host = "http://localhost:3003"

// 用于复现 sparkdest 模型在高并发时可能出现的 bug，不过这个测试也很难复现出来
func TestSparkDesk(t *testing.T) {
	url := host + "/v1/chat/completions"
	method := "POST"

	client := &http.Client{}
	// 以并发6 来测试验证 bug
	wg := &sync.WaitGroup{}
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			payload := strings.NewReader(`{
    "model": "SparkDesk",
    "temperature": 0.5,
    "max_tokens": 1024,
    "stream": true,
    "messages": [
        {"role": "user", "content": "你是谁, 你会做什么"}
    ]
}`)

			req, err := http.NewRequest(method, url, payload)

			if err != nil {
				fmt.Println(err)
				return
			}
			req.Header.Add("Authorization", "Bearer sk-cZxxu3d73fvZNGouEe8129C7E0254f33932d89E4B5E92b18")
			req.Header.Add("Content-Type", "application/json")

			res, err := client.Do(req)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer res.Body.Close()

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				fmt.Println(err)
				wg.Done()
				return
			}
			fmt.Println(string(body))
			wg.Done()
		}()
	}
	wg.Wait()
}
