package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

const host = "http://localhost:3003"

func login() (*http.Client, error) {
	// 创建一个 cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	// 创建一个客户端，并指定使用 cookie jar
	client := &http.Client{
		Jar: jar,
	}

	req := LoginRequest{
		Username: "root",
		Password: "123456",
	}
	reqBytes, _ := json.Marshal(req)
	// 第一次请求，发送一个设置了 cookie 的响应
	resp1, err := client.Post(host+"/api/user/login", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		panic(err)
	}

	// 输出 cookie
	for _, cookie := range resp1.Cookies() {
		fmt.Printf("Got cookie: %v\n", cookie)
	}
	return client, nil
}

func TestAutoDisableForUnauthorized(t *testing.T) {
	url := host + "/v1/chat/completions"
	method := "POST"

	payload := strings.NewReader(`{
    "model": "gpt-3.5-turbo",
    "temperature": 0.5,
    "max_tokens": 1024,
    "stream": true,
    "messages": [
        {"role": "user", "content": "你是谁，你会做什么"}
    ]
}`)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Add("Authorization", "Bearer sk-lgEExg41M1VCcYpUDc34DaC4D545403596B9389cAc64Fe83")
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
		return
	}
	fmt.Println(string(body))

	loginedClient, err := login()
	assert.Nil(t, err)

	resp, err := loginedClient.Get(host + "/api/channel/1")
	assert.Nil(t, err)
	defer resp.Body.Close()

	channelRespMap := make(map[string]interface{})
	channelRespBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(channelRespBytes, &channelRespMap)
	success, ok := channelRespMap["success"].(bool)
	assert.True(t, success && ok)
	dataMap, _ := channelRespMap["data"].(map[string]interface{})
	status := dataMap["status"].(float64)
	assert.Equal(t, float64(0), status)
}
