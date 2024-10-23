package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]*ssh.Client)
var mutex = sync.Mutex{}

// logError 统一的错误处理函数
func logError(err error, msg string) {
	if err != nil {
		log.Printf("%s: %v\n", msg, err)
	}
}

// handleClient 处理 WebSocket 客户端连接
func handleClient(conn *websocket.Conn, ssh SSHConn) {
	defer func() {
		mutex.Lock()
		if client, ok := clients[conn]; ok {
			client.Close() // 关闭 SSH 连接
			delete(clients, conn)
		}
		mutex.Unlock()
		conn.Close()
		log.Printf("Client disconnected: %s\n", conn.RemoteAddr().String())
	}()

	log.Printf("Client connected: %s\n", conn.RemoteAddr().String())
	client, err := connSSH(ssh)
	if err != nil {
		logError(err, "SSH Connection Error")
		return
	}

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			logError(err, "WebSocket Read Error")
			break
		}

		if msgType == websocket.TextMessage {
			var receiveMsg ReceiveMsg
			log.Println("Received message:", string(msg)) // 打印接收到的消息以便调试
			err = json.Unmarshal(msg, &receiveMsg)
			if err != nil {
				logError(err, "JSON Unmarshal Error")
				continue
			}

			// 执行 SSH 命令，设置超时
			output, err := executeSSHCommandWithTimeout(client, receiveMsg.Command, 10*time.Second)
			if err != nil {
				logError(err, "SSH Command Execution Error")
				output = err.Error() // 记录错误信息
			}

			log.Println("Command output:", output)

			// 将命令结果发送回客户端
			response := NewSendMsg(receiveMsg.UserId, output)
			responseBytes, err := json.Marshal(response)
			if err != nil {
				logError(err, "JSON Marshal Error")
				continue
			}

			err = conn.WriteMessage(websocket.TextMessage, responseBytes)
			if err != nil {
				logError(err, "WebSocket Write Error")
				break
			}
		} else {
			log.Println("Unsupported message type:", msgType)
		}
	}
}

// executeSSHCommandWithTimeout 执行 SSH 命令并设置超时
func executeSSHCommandWithTimeout(client *ssh.Client, command string, timeout time.Duration) (string, error) {
	// 创建 SSH 会话
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// 使用 context 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		// 超时情况处理
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			return stderr.String(), err
		}
		return stdout.String(), nil
	}
}

// handleWebSocket 升级 HTTP 连接到 WebSocket
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	userId := r.URL.Query().Get("id")
	if userId == "" {
		http.Error(w, "Missing 'ssh_id' parameter", http.StatusBadRequest)
		return
	}
	ssh, err := getSSHConfig(userId)
	if err != nil {
		http.Error(w, "Failed to get SSH config", http.StatusInternalServerError)
		return
	}
	log.Println(ssh)

	conn, err := upgrader.Upgrade(w, r, nil)
	logError(err, "WebSocket Upgrade Error")
	if err != nil {
		return
	}

	go handleClient(conn, ssh)
}

// main 启动 WebSocket 服务器
func RunWebsocket() {
	http.HandleFunc("/ws", handleWebSocket)
	log.Println("Server running on :800")
	err := http.ListenAndServe(":800", nil)
	logError(err, "Server Error")
}

// SSH 连接处理相关的结构体和方法
type SendMsg struct {
	UserId   string `json:"user_id"`
	Response string `json:"response"`
}

type ReceiveMsg struct {
	UserId  string `json:"user_id"`
	Command string `json:"command"`
}

func NewSendMsg(userId, response string) *SendMsg {
	return &SendMsg{
		UserId:   userId,
		Response: response,
	}
}

// connByCrypto 使用私钥连接 SSH
func connectSSH(conf SSHConn) (*ssh.Client, error) {
	if conf.PrivateKey != "" {
		// 将私钥字符串解析为 ssh.Signer
		signer, err := ssh.ParsePrivateKey([]byte(conf.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}

		config := &ssh.ClientConfig{
			User: conf.Username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		host := fmt.Sprintf("%s:%s", conf.Host, conf.Port)
		client, err := ssh.Dial("tcp", host, config)
		return client, err
	}
	return nil, fmt.Errorf("private key is not valid")
}
func connSSH(conf SSHConn) (*ssh.Client, error) {
	log.Printf("conf %s", conf)
	if conf.Password != "" {
		config := &ssh.ClientConfig{
			User: conf.Username,
			Auth: []ssh.AuthMethod{
				ssh.Password(conf.Password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		host := fmt.Sprintf("%s:%s", conf.Host, conf.Port)
		client, err := ssh.Dial("tcp", host, config)
		return client, err
	} else if conf.PrivateKey != "" {

		signer, err := ssh.ParsePrivateKey([]byte(conf.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}

		config := &ssh.ClientConfig{
			User: conf.Username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		host := fmt.Sprintf("%s:%s", conf.Host, conf.Port)
		client, err := ssh.Dial("tcp", host, config)
		return client, err

	} else {
		return nil, fmt.Errorf("password is not valid")
	}

}

type Res struct {
	Message string  `json:"message"`
	Data    SSHConn `json:"data"`
}

// getSSHConfig 从后端 API 获取 SSH 配置信息
func getSSHConfig(id string) (SSHConn, error) {
	var res Res
	var sshConn SSHConn
	// 将 id 从字符串转换为整数
	intID, err := strconv.Atoi(id)
	if err != nil {
		return sshConn, fmt.Errorf("invalid id: %v", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:8000/api/connect-ssh/%d", intID)

	resp, err := http.Get(url)
	if err != nil {
		return sshConn, fmt.Errorf("failed to make GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return sshConn, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return sshConn, fmt.Errorf("failed to read response body: %v", err)
	}

	err = json.Unmarshal(body, &res)
	if err != nil {
		return sshConn, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}
	sshConn = res.Data

	return sshConn, nil
}
