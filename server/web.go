package server

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// 自定义日志格式化函数
func logFormat(param gin.LogFormatterParams) string {
	return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
		param.ClientIP,
		param.TimeStamp.Format(time.RFC3339Nano),
		param.Method,
		param.Path,
		param.Request.Proto,
		param.StatusCode,
		param.Latency,
		param.Request.UserAgent(),
		param.ErrorMessage,
	)
}

func initDB() {
	var err error
	dsn := "root:qweasdzxc@tcp(127.0.0.1:3306)/go"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)             //设置最大打开连接数为 10。
	db.SetMaxIdleConns(5)              //设置最大空闲连接数为 5。
	db.SetConnMaxLifetime(5 * 60 * 60) // 设置连接的最大生命周期为 5 小时。

	// 测试数据库连接
	err = db.Ping()
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
	log.Println("Database connection established")
}

func RunWebServer() {
	// 初始化数据库连接
	initDB()
	// 设置日志输出到标准输出
	log.SetOutput(os.Stdout)

	// 创建 Gin 引擎并使用自定义日志格式化函数
	r := gin.New()
	// 设置 CORS 中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"POST", "GET", "OPTIONS", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.Use(gin.LoggerWithFormatter(logFormat))
	r.Use(gin.Recovery())
	r.GET("/ping", hello_handler)
	r.POST("/api/ssh", post_handler)
	r.DELETE("/api/delete-ssh/:id", delete_ssh_handle)
	r.GET("/api/list-ssh", list_ssh_handle)
	r.GET("/api/connect-ssh/:id", connectHandle)

	r.Run(":8000")
}

func hello_handler(c *gin.Context) {
	log.Println("INFO: Received request for /ping")
	c.JSON(200, gin.H{
		"message": "Hello, Gin!",
	})
}

func post_handler(c *gin.Context) {
	// 获取 JSON 数据
	var requestData struct {
		AuthMethod string `json:"authMethod"`
		Password   string `json:"password"`
		Host       string `json:"host"`
		Port       string `json:"port"`
		PrivateKey string `json:"privateKey"`
		Username   string `json:"username"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		log.Printf("ERROR: 解析 JSON 数据失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "解析 JSON 数据失败",
		})
		return
	}

	authway := c.Query("authway")

	// 根据 authway 参数判断是上传密钥还是使用密码
	if authway == "key" {
		if requestData.PrivateKey == "" {
			log.Printf("ERROR: 私钥不能为空")
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "私钥不能为空",
			})
			return
		}

		// 处理使用密钥的逻辑
		log.Println("INFO: 使用密钥进行认证")
		log.Printf("INFO: host=%s, port=%s, user=%s, privateKey=%s, username=%s\n", requestData.Host, requestData.Port, requestData.Username, requestData.PrivateKey, requestData.Username)

		//存储
		_, err := db.Exec("INSERT INTO ssh(host, port, user, private_key) VALUES(?, ?, ?, ?)",
			requestData.Host, requestData.Port, requestData.Username, requestData.PrivateKey)
		if err != nil {
			log.Printf("ERROR: 数据库插入失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "数据库插入失败",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "add ssh success",
		})
	} else {
		if requestData.Password == "" {
			log.Printf("ERROR: 密码不能为空")
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "密码不能为空",
			})
			return
		}

		// 处理使用密码的逻辑
		log.Println("INFO: 使用密码进行认证")
		log.Printf("INFO: host=%s, port=%s, user=%s, password=%s, username=%s\n", requestData.Host, requestData.Port, requestData.Username, requestData.Password, requestData.Username)
		_, err := db.Exec("INSERT INTO ssh(host, port, user, password) VALUES(?, ?, ?, ?)",
			requestData.Host, requestData.Port, requestData.Username, requestData.Password)
		if err != nil {
			log.Printf("ERROR: 数据库插入失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "数据库插入失败",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "add ssh success",
		})
	}
}

func delete_ssh_handle(c *gin.Context) {
	id := c.Param("id")
	_, err := db.Exec("DELETE FROM ssh WHERE id=?", id)
	if err != nil {
		log.Printf("ERROR: 数据库删除失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "数据库删除失败",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "delete ssh success",
	})
}
func list_ssh_handle(c *gin.Context) {
	rows, err := db.Query("SELECT * FROM ssh")
	if err != nil {
		log.Printf("ERROR: 数据库查询失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "数据库查询失败",
		})
		return
	}
	defer rows.Close()

	var sshList []map[string]interface{}
	for rows.Next() {
		var id int
		var host, port, user string
		var password, private_key sql.NullString

		err := rows.Scan(&id, &host, &port, &user, &password, &private_key)
		if err != nil {
			log.Printf("ERROR: 数据库查询结果解析失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "数据库查询结果解析失败",
			})
			return
		}

		// 构建 ssh 连接信息
		ssh := map[string]interface{}{
			"id":   id,
			"host": host,
			"port": port,
			"user": user,
		}

		// 根据 password 和 private_key 的值确定 auth_method
		if password.Valid {
			ssh["auth_method"] = "password"
		} else if private_key.Valid {
			ssh["auth_method"] = "key"
		} else {
			ssh["auth_method"] = "unknown" // 如果都为空
		}

		sshList = append(sshList, ssh)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "list ssh success",
		"data":    sshList,
	})
}

// connectSSH 处理 SSH 连接请求
func connectHandle(c *gin.Context) {
	id := c.Params.ByName("id")

	// 连接数据库获取 SSH 信息
	var ssh struct {
		ID         int
		Host       string
		Port       string
		Username   string
		Password   sql.NullString
		PrivateKey sql.NullString
	}

	// 查询数据库获取 SSH 信息
	err := db.QueryRow("SELECT id, host, port, user, password, private_key FROM ssh WHERE id=?", id).Scan(
		&ssh.ID, &ssh.Host, &ssh.Port, &ssh.Username, &ssh.Password, &ssh.PrivateKey)

	if err != nil {
		if err == sql.ErrNoRows {
			// 处理没有找到记录的情况
			log.Printf("INFO: No SSH record found for ID: %s", id)
			c.JSON(http.StatusNotFound, gin.H{
				"message": "No SSH record found",
			})
		} else {
			// 处理其他错误
			log.Printf("ERROR: 数据库查询失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "数据库查询失败",
			})
		}
		return
	}

	// 创建返回给前端的 SSHConn 实例
	sshConn := SSHConn{
		ID:       ssh.ID,
		Host:     ssh.Host,
		Port:     ssh.Port,
		Username: ssh.Username,
	}

	// 处理 Password 和 PrivateKey 的 NULL 值
	if ssh.Password.Valid {
		sshConn.Password = ssh.Password.String
	}
	if ssh.PrivateKey.Valid {
		sshConn.PrivateKey = ssh.PrivateKey.String
	}

	// 返回 JSON 数据给前端
	c.JSON(http.StatusOK, gin.H{
		"message": "connect ssh success",
		"data":    sshConn,
	})
}
