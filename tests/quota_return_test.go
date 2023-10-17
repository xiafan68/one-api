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

type User struct {
	Id               int    `json:"id"`
	Username         string `json:"username" gorm:"unique;index" validate:"max=12"`
	Password         string `json:"password" gorm:"not null;" validate:"min=8,max=20"`
	DisplayName      string `json:"display_name" gorm:"index" validate:"max=20"`
	Role             int    `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status           int    `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Email            string `json:"email" gorm:"index" validate:"max=50"`
	GitHubId         string `json:"github_id" gorm:"column:github_id;index"`
	WeChatId         string `json:"wechat_id" gorm:"column:wechat_id;index"`
	VerificationCode string `json:"verification_code" gorm:"-:all"`                                    // this field is only for Email verification, don't save it to database!
	AccessToken      string `json:"access_token" gorm:"type:char(32);column:access_token;uniqueIndex"` // this token is for system management
	Quota            int    `json:"quota" gorm:"type:int;default:0"`
	UsedQuota        int    `json:"used_quota" gorm:"type:int;default:0;column:used_quota"` // used quota
	RequestCount     int    `json:"request_count" gorm:"type:int;default:0;"`               // request number
	Group            string `json:"group" gorm:"type:varchar(32);default:'default'"`
	AffCode          string `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	InviterId        int    `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
}

const host = "http://localhost:3003"

func login(user, password string) (*http.Client, error) {
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
		Username: user,
		Password: password,
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

func unmarshalData(body io.Reader, v interface{}) error {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	respMsg := make(map[string]interface{})
	err = json.Unmarshal(bodyBytes, &respMsg)
	if err != nil {
		return err
	}
	dataBytes, _ := json.Marshal(respMsg["data"])
	return json.Unmarshal(dataBytes, v)
}

func TestReturnQuotaWhenUpStreamLLMFailed(t *testing.T) {
	authedClient, _ := login("quota_return", "quota_return")
	resp, err := authedClient.Get(host + "/api/user/self")
	preUser := User{}
	unmarshalData(resp.Body, &preUser)

	url := host + "/v1/chat/completions"
	method := "POST"

	// this model will failed with 401
	payload := strings.NewReader(`{
    "model": "gpt-3.5-turbo-0301",
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
	req.Header.Add("Authorization", "Bearer sk-GU5b5sytVc12QtvlCd77723835D149478fE378AcFe01DeDb")
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	assert.Nil(t, err)
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
	resp, err = authedClient.Get(host + "/api/user/self")
	newUser := &User{}
	unmarshalData(resp.Body, newUser)
	assert.Equal(t, preUser.Quota, newUser.Quota)
}
