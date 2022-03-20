package main

import (
	"embed"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
)

//下面这句话的意思是打包go的时候把后面这个目录打包进去
//go:embed frontend/dist/*
var FS embed.FS

func main() {
	go func() {
		gin.SetMode(gin.DebugMode)
		router := gin.Default()
		//把打包好的静态文件变成一个结构化的目录
		staticFiles, _ := fs.Sub(FS, "frontend/dist")
		router.POST("/api/v1/texts", TextsController)
		router.GET("/api/v1/addresses", AddressesController)
		router.StaticFS("/static", http.FS(staticFiles))
		//NoRoute表示用户访问路径没匹配到程序定义的路由
		router.NoRoute(func(c *gin.Context) {
			//获取用户访问的路径
			path := c.Request.URL.Path
			//判断路径是否以static开头
			if strings.HasPrefix(path, "/static/") {
				reader, err := staticFiles.Open("index.html")
				if err != nil {
					log.Fatal(err)
				}
				defer reader.Close()
				stat, err := reader.Stat()
				if err != nil {
					log.Fatal(err)
				}
				c.DataFromReader(http.StatusOK, stat.Size(), "text/html", reader, nil)
				//如果不是static开头则返回404
			} else {
				c.Status(http.StatusNotFound)
			}
		})
		router.Run(":8080")
	}()
	chSignal := make(chan os.Signal, 1)
	//signal.Notify订阅os.Interrupt信号,一旦有信号则往chSingal管道里写一个信号
	signal.Notify(chSignal, os.Interrupt)

	chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	var tmpDir string
	name, _ := ioutil.TempDir("", "lorca")
	tmpDir = name
	//删除缓存文件
	defer os.RemoveAll(tmpDir)
	fmt.Println(tmpDir)
	cmd := exec.Command(chromePath, "--app=http://localhost:8080/static/index.html", fmt.Sprintf("--user-data-dir=%s", tmpDir),
		"--no-first-run")
	cmd.Start()
	//如果没有值则一直等待（阻塞），直到有信号输入
	//select可以监听多个管道，只要有一个管道有信号则进行下一步
	//如果没有信号，select就等待（阻塞）
	select {
	case <-chSignal:
		//一旦有信号则关闭浏览器
		cmd.Process.Kill()
	}
}

func TextsController(c *gin.Context) {
	var json struct {
		Raw string `json:"raw"`
	}
	//将获取到的数据传给json变量
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		//获取当前目录
		exe, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		dir := filepath.Dir(exe)
		if err != nil {
			log.Fatal(err)
		}
		//随机生成一个字符串并复制给filename，用来做上传后的文件名
		filename := uuid.New().String()
		//拼接uploads的绝对路径
		uploads := filepath.Join(dir, "uploads")
		//创建uploads目录
		err = os.MkdirAll(uploads, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		//合成上传后文件的路径
		fullpath := path.Join("uploads", filename+".txt")
		//把json.Raw写到文件里
		err = ioutil.WriteFile(filepath.Join(dir, fullpath), []byte(json.Raw), 0644)
		if err != nil {
			log.Fatal(err)
		}
		//返回文件的路径到texts接口的http respond，比如/uploads/c07b266c-53ce-435d-91d0-bb4cbbb00ecb.txt
		c.JSON(http.StatusOK, gin.H{"url": "/" + fullpath})
	}
}

func AddressesController(c *gin.Context) {
	//获取当前电脑的所有ip地址
	addrs, _ := net.InterfaceAddrs()
	var result []string
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		// 断言address里的地址是ip地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				result = append(result, ipnet.IP.String())
			}
		}
	}
	//转为json写入address接口的http respond
	c.JSON(http.StatusOK, gin.H{"addresses": result})
}
