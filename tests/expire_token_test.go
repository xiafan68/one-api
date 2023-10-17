package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

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

type Token struct {
	Id             int    `json:"id"`
	UserId         int    `json:"user_id"`
	Key            string `json:"key" gorm:"type:char(48);uniqueIndex"`
	Status         int    `json:"status" gorm:"default:1"`
	Name           string `json:"name" gorm:"index" `
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
	AccessedTime   int64  `json:"accessed_time" gorm:"bigint"`
	ExpiredTime    int64  `json:"expired_time" gorm:"bigint;default:-1"` // -1 means never expired
	RemainQuota    int    `json:"remain_quota" gorm:"default:0"`
	UnlimitedQuota bool   `json:"unlimited_quota" gorm:"default:false"`
	UsedQuota      int    `json:"used_quota" gorm:"default:0"` // used quota
}

const host = "http://localhost:3003"

func login(user, password string) (*http.Client, error) {
	// 创建 token
	//
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

func TestEnableExpireToken(t *testing.T) {
	authedClient, _ := login("expire_test", "expire_test")
	resp, _ := authedClient.Get(host + "/api/user/self")
	user := User{}
	unmarshalData(resp.Body, &user)

	newToken := Token{}
	req, _ := http.NewRequest("GET", host+"/api/token/5", nil)
	res, _ := authedClient.Do(req)
	unmarshalData(res.Body, &newToken)

	// 修改状态，并且发送一个请求
	// 修改过期时间
	newToken.ExpiredTime = time.Now().Unix() + 1
	newTokenBytes, _ := json.Marshal(newToken)
	req, _ = http.NewRequest("PUT", host+"/api/token", bytes.NewBuffer(newTokenBytes))
	authedClient.Do(req)
	newToken.Status = 1
	newTokenBytes, _ = json.Marshal(newToken)
	req, _ = http.NewRequest("PUT", host+"/api/token?status_only=true", bytes.NewBuffer(newTokenBytes))
	res, _ = authedClient.Do(req)

	time.Sleep(time.Second * 3)
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
	req.Header.Add("Authorization", "Bearer "+newToken.Key)
	req.Header.Add("Content-Type", "application/json")
	res, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	// 设置为永不过期在重新启用 token，检查结果
	newToken.ExpiredTime = -1
	newToken.Status = 3
	newTokenBytes, _ = json.Marshal(newToken)
	req, _ = http.NewRequest("PUT", host+"/api/token", bytes.NewBuffer(newTokenBytes))
	res, err = authedClient.Do(req)
	newToken.Status = 1
	newTokenBytes, _ = json.Marshal(newToken)
	req, _ = http.NewRequest("PUT", host+"/api/token?status_only=true", bytes.NewBuffer(newTokenBytes))
	res, _ = authedClient.Do(req)
	req, _ = http.NewRequest("GET", host+"/api/token/5", nil)
	res, _ = authedClient.Do(req)
	unmarshalData(res.Body, &newToken)
	assert.Equal(t, 1, newToken.Status)
}
