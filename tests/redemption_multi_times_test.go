package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Redemption struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id"`
	Key          string `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int    `json:"status" gorm:"default:1"`
	Name         string `json:"name" gorm:"index"`
	Quota        int    `json:"quota" gorm:"default:100"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	RedeemedTime int64  `json:"redeemed_time" gorm:"bigint"`
	Count        int    `json:"count" gorm:"-:all"` // only for api request
}

type TopUpRequest struct {
	Key string `json:"key"`
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

const host = "http://localhost:3000"

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

func TestRedemptionUsedMultiTimes(t *testing.T) {
	// 添加兑换码： post /api/redemption
	// 进行兑换：post /api/user/topup
	authedClient, _ := login("credit_test", "credit_test")
	resp, err := authedClient.Get(host + "/api/user/self")

	preUser := User{}
	unmarshalData(resp.Body, &preUser)

	rootClient, _ := login("root", "123456")
	url := host + "/api/redemption"
	method := "POST"
	redemption := Redemption{
		UserId: 1,
		Status: 1,
		Name:   "测试兑换码",
		Quota:  10000,
		Count:  1,
	}
	redemptionBytes, _ := json.Marshal(redemption)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(redemptionBytes))

	if err != nil {
		fmt.Println(err)
		return
	}

	res, err := rootClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	respMap := make(map[string]interface{})
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	json.Unmarshal(body, &respMap)
	success, ok := respMap["success"].(bool)
	assert.Equal(t, true, success && ok)
	keys, _ := respMap["data"].([]interface{})
	assert.Equal(t, 1, len(keys))
	key := keys[0].(string)

	wg := sync.WaitGroup{}
	topUpRequest := TopUpRequest{Key: key}
	topUpBytes, _ := json.Marshal(topUpRequest)
	errCount := 0
	parallelCount := 20
	for i := 0; i < parallelCount; i++ {
		wg.Add(1)
		go func() {
			resp, _ := authedClient.Post(host+"/api/user/topup", "application/json", bytes.NewBuffer(topUpBytes))
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			msg := make(map[string]interface{})
			json.Unmarshal(bodyBytes, &msg)
			success, _ := msg["success"].(bool)
			if !success {
				errCount++
			}
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, parallelCount-1, errCount)

	// 验证账户金额只加了一次
	resp, err = authedClient.Get(host + "/api/user/self")

	newUser := User{}
	unmarshalData(resp.Body, &newUser)
	assert.Equal(t, preUser.Quota+redemption.Quota, newUser.Quota)
}
