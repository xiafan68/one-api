package spark_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestSparkDesk(t *testing.T) {
	url := "http://100.80.248.106:3000/v1/chat/completions"
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
			req.Header.Add("Authorization", "Bearer sk-GKG74fcJAQuHXGRmD08245E9457c4b05989460063fD13366")
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
