package server

// 定义 SSHConn 结构体，用于返回给前端
type SSHConn struct {
	ID         int    `json:"id"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`    // 如果密码为空则不包含该字段
	PrivateKey string `json:"private_key,omitempty"` // 如果私钥为空则不包含该字段
}
