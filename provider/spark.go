package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"kandaoni.com/anqicms/config"
	"log"
	"net/url"
	"strings"
	"time"
)

type SparkMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var sparkApiUrls = map[string]string{
	"1.5": "wss://spark-api.xf-yun.com/v1.1/chat",
	"3.0": "wss://spark-api.xf-yun.com/v3.1/chat",
	"3.5": "wss://spark-api.xf-yun.com/v3.5/chat",
	"4.0": "wss://spark-api.xf-yun.com/v4.0/chat",
	"pro": "wss://spark-api.xf-yun.com/chat/pro-128k",
}

var ErrSensitive = errors.New("sensitive")

func GetSparkResponse(sparkKey config.SparkSetting, prompt string) (string, error) {
	buf, err := GetSparkStream(sparkKey, prompt)
	if err != nil {
		if strings.Contains(err.Error(), "非常抱歉，根据相关法律法规，我们无法提供关于以下内容的答案") {
			return "", ErrSensitive
		}
		return "", err
	}
	var answer string
	for {
		line, err2 := <-buf
		var isEof bool
		if err2 == false {
			log.Println("false", err2)
			log.Println("is eof", errors.Is(err, io.EOF))
			isEof = true
		}

		if line == "EOF" {
			break
		}
		if len(line) > 1 {
			answer += line
		}
		if isEof {
			break
		}
	}
	if len(answer) == 0 {
		return "", errors.New("无可用内容")
	}
	if strings.HasPrefix(answer, "非常抱歉") || strings.Contains(answer, "非常抱歉，根据相关法律法规，我们无法提供关于以下内容的答案") {
		return "", ErrSensitive
	}

	return answer, nil
}

func GetSparkStream(sparkKey config.SparkSetting, prompt string) (chan string, error) {
	apiHost, ok := sparkApiUrls[sparkKey.Version]
	if !ok {
		return nil, errors.New("未选择模型版本")
	}
	d := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	//握手并建立websocket 连接
	conn, resp, err := d.Dial(assembleAuthUrl1(apiHost, sparkKey.APIKey, sparkKey.APISecret), nil)
	if err != nil {
		log.Println("dial err", err)
		return nil, err
	}
	if resp.StatusCode != 101 {
		b, err2 := io.ReadAll(resp.Body)
		if err2 == nil {
			log.Println("err", string(b))
		}
		log.Println("err", resp.StatusCode)
		return nil, errors.New(resp.Status)
	}

	go func() {
		data := genParams1(sparkKey.AppID, prompt, sparkKey.Version)
		conn.WriteJSON(data)
	}()

	var buf = make(chan string, 1)

	go func() {
		//获取返回的数据
		for {
			_, msg, err1 := conn.ReadMessage()
			if err1 != nil {
				fmt.Println("read message error:", err1)
				err = err1
				buf <- "EOF"
				break
			}

			var data map[string]interface{}
			err1 = json.Unmarshal(msg, &data)
			if err1 != nil {
				fmt.Println("Error parsing JSON:", err1)
				err = err1
				buf <- "EOF"
				return
			}
			//解析数据
			payload, ok := data["payload"].(map[string]interface{})
			if !ok {
				fmt.Printf("Error payload:%#v", data)
				fmt.Printf("%+v", prompt)
				message := "error message"
				header, ok := data["header"].(map[string]interface{})
				if ok {
					message, _ = header["message"].(string)
				}
				err = errors.New(message)
				buf <- "EOF"
				return
			}
			choices, ok := payload["choices"].(map[string]interface{})
			if !ok {
				fmt.Printf("Error choices:%#v", data)
				err = errors.New("error choices")
				buf <- "EOF"
				return
			}
			header, ok := data["header"].(map[string]interface{})
			if !ok {
				fmt.Printf("Error header:%#v", data)
				err = errors.New("error header")
				buf <- "EOF"
				return
			}
			code, ok := header["code"].(float64)
			if !ok {
				fmt.Printf("Error code:%#v", data)
				err = errors.New("error code")
				buf <- "EOF"
				return
			}
			if code != 0 {
				fmt.Println(data["payload"])
				err = errors.New("code error")
				buf <- "EOF"
				return
			}
			status := choices["status"].(float64)
			text := choices["text"].([]interface{})
			content := text[0].(map[string]interface{})["content"].(string)

			buf <- content
			if status == 2 {
				usage := payload["usage"].(map[string]interface{})
				temp := usage["text"].(map[string]interface{})
				totalTokens := temp["total_tokens"].(float64)
				fmt.Println("total_tokens:", totalTokens)
				conn.Close()
				break
			}
		}
		//输出返回结果
		buf <- "EOF"
	}()
	time.Sleep(1 * time.Second)

	return buf, err
}

// 生成参数
func genParams1(appid, question string, ver string) map[string]interface{} { // 根据实际情况修改返回的数据结构和字段名

	messages := []SparkMessage{
		{Role: "user", Content: question},
	}
	domain := "general"
	if ver == "2.0" {
		domain = "generalv2"
	} else if ver == "3.0" {
		domain = "generalv3"
	} else if ver == "3.5" {
		domain = "generalv3.5"
	} else if ver == "pro" {
		domain = "pro-128k"
	} else if ver == "4.0" {
		domain = "4.0Ultra"
	}

	data := map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
		"header": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
			"app_id": appid, // 根据实际情况修改返回的数据结构和字段名
		},
		"parameter": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
			"chat": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
				"domain":      domain,       // 根据实际情况修改返回的数据结构和字段名
				"temperature": float64(0.8), // 根据实际情况修改返回的数据结构和字段名
				"top_k":       int64(6),     // 根据实际情况修改返回的数据结构和字段名
				"max_tokens":  int64(2048),  // 根据实际情况修改返回的数据结构和字段名
				"auditing":    "default",    // 根据实际情况修改返回的数据结构和字段名
			},
		},
		"payload": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
			"message": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
				"text": messages, // 根据实际情况修改返回的数据结构和字段名
			},
		},
	}
	return data // 根据实际情况修改返回的数据结构和字段名
}

// 创建鉴权url  apikey 即 hmac username
func assembleAuthUrl1(hosturl string, apiKey, apiSecret string) string {
	ul, err := url.Parse(hosturl)
	if err != nil {
		fmt.Println(err)
	}
	//签名时间
	date := time.Now().UTC().Format(time.RFC1123)
	//date = "Tue, 28 May 2019 09:10:42 MST"
	//参与签名的字段 host ,date, request-line
	signString := []string{"host: " + ul.Host, "date: " + date, "GET " + ul.Path + " HTTP/1.1"}
	//拼接签名字符串
	sgin := strings.Join(signString, "\n")
	// fmt.Println(sgin)
	//签名结果
	sha := HmacWithShaTobase64("hmac-sha256", sgin, apiSecret)
	// fmt.Println(sha)
	//构建请求参数 此时不需要urlencoding
	authUrl := fmt.Sprintf("hmac username=\"%s\", algorithm=\"%s\", headers=\"%s\", signature=\"%s\"", apiKey,
		"hmac-sha256", "host date request-line", sha)
	//将请求参数使用base64编码
	authorization := base64.StdEncoding.EncodeToString([]byte(authUrl))

	v := url.Values{}
	v.Add("host", ul.Host)
	v.Add("date", date)
	v.Add("authorization", authorization)
	//将编码后的字符串url encode后添加到url后面
	callurl := hosturl + "?" + v.Encode()
	return callurl
}

func HmacWithShaTobase64(algorithm, data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	encodeData := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(encodeData)
}
