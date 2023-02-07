package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	_url     = flag.String("url", "", "m3u8 url.")
	_file    = flag.String("file", "", "m3u8 file path.")
	_host    = flag.String("host", "", "host prefix.")
	_co      = flag.String("co", "", "goroutine total.")
	_output  = flag.String("output", "", "output dir.")
	dataDir  = "output/"
	wg       = &sync.WaitGroup{}
	ch       chan int
	urlQueue *queue
)

func main() {
	flag.Parse()
	var (
		urlInfo *url.URL
		tsList  string
	)

	coTotal, err := strconv.Atoi(*_co)
	if len(*_url) <= 0 && len(*_file) <= 0 {
		panic("Parameters `url` and `file` cannot be both empty")
	}

	if err != nil || coTotal < 1 {
		coTotal = 5
	}

	if coTotal > 1000 {
		panic("The goroutine maximum is 1000")
	}
	// 创建数据目录
	if len(*_output) > 0 {
		dataDir = *_output
	}

	if !PathExists(dataDir) {
		os.MkdirAll(dataDir, os.ModePerm)
	}

	if len(*_url) > 0 {
		urlInfo, _ = url.Parse(*_url)
		resp, err := http.Get(*_url)
		if err != nil {
			panic(err)
		}
		body, _ := io.ReadAll(resp.Body)
		tsList = string(body)
	} else if PathExists(*_file) {
		urlInfo, _ = url.Parse(*_host)
		if urlInfo.Host == "" {
			panic("file host empty")
		}
		_bytes, err := os.ReadFile(*_file)
		if err != nil {
			panic(err)
		}
		tsList = string(_bytes)
	} else {
		panic("Invalid data")
	}
	flagMatch, _ := regexp.MatchString("^#EXTM3U", tsList);
	if !flagMatch {
		panic("Invalid data")
	}
	// 删除注释
	commentLineRegexp, err := regexp.Compile("#.+\n*")
	if err == nil {
		tsList = commentLineRegexp.ReplaceAllString(tsList, "")
	}
	tsList = strings.TrimSpace(tsList)
	// 每行一个文件
	lines := strings.Split(tsList, "\n")
	// 队列，让所有协程竞争这个队列数据
	urlQueue = &queue{
		items: lines,
	}
	// 创建协程
	for i := 1; i <= coTotal; i++ {
		wg.Add(1)
		go saveProc(*urlInfo)
	}
	// 阻塞主线程，等所有协程执行完再往下执行
	wg.Wait()
	fmt.Println(">", "All goroutine done")
	// 生成ffmpeg文件索引文件
	mergeTxt := ""
	for _, line := range lines {
		filename := path.Base(line)
		mergeTxt += "file '" + filename + "'\n"
	}
	os.WriteFile(dataDir+"/merge.txt", []byte(mergeTxt), fs.ModePerm)
	fmt.Println("")
	fmt.Println(">", "Run the following command to generate an mp4 file:")
	fmt.Println("ffmpeg -f concat -i " + strings.TrimSuffix(dataDir, "/") + "/merge.txt -c copy output.mp4")
}

// 协程函数
func saveProc(urlInfo url.URL) {
	id := rand.Intn(999999)
	for urlQueue.length() > 0 {
		tsName := strings.TrimSpace(urlQueue.Pop()) // 让每个协程每次下载一个ts文件
		if tsName == "" {
			return
		}
		tsUrl := urlUnparse(tsName, urlInfo)
		result, _ := saveTsFile(tsUrl, tsName)
		fmt.Println(">", id, tsUrl, result)
		if !result {
			urlQueue.Push(tsName)
		}
	}
	//fmt.Println(id, "complete")
	wg.Done()
}

// url合成
func urlUnparse(filename string, urlInfo url.URL) string {
	match, _ := regexp.MatchString("^http[s]*://", filename)
	if match {
		return filename
	}
	userpart := ""
	if len(urlInfo.User.Username()) > 0 {
		pwd, _ := urlInfo.User.Password()
		userpart = urlInfo.User.Username() + "@" + pwd
	}
	if string(filename[0]) == "/" {
		return urlInfo.Scheme + "://" + userpart + "" + urlInfo.Host + filename
	}
	return urlInfo.Scheme + "://" + userpart + "" + urlInfo.Host + path.Dir(urlInfo.Path) + "/" + filename
}

// 保存文件
func saveTsFile(url string, filename string) (bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return false, err
	} else if resp.StatusCode != 200 {
		return false, errors.New("HTTP CODE: " + fmt.Sprintf("%d", resp.StatusCode))
	}
	buffer, err := io.ReadAll(resp.Body)
	if err != nil || len(buffer) <= 0 {
		return false, errors.New("fail to read buffer ")
	}
	filename = path.Base(filename)
	file, err := os.Create(dataDir + "/" + filename)
	if err != nil {
		return false, err
	}
	n, err := file.Write(buffer)
	if err != nil || n <= 0 {
		return false, errors.New("write fail")
	}
	return true, nil
}

// 判断路径是否存在
func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return false
}

// 队列
type queue struct {
	// 互斥锁
	sync.Mutex
	items []string
}

// 右入队列
func (this *queue) Push(items ...string) bool {
	this.Lock()
	defer this.Unlock()
	this.items = append(this.items, items...)
	return true
}

// 左入队列
func (this *queue) Unshift(items ...string) bool {
	this.Lock()
	defer this.Unlock()
	readyItems := make([]string, len(items))
	this.items = append(readyItems, this.items...)
	return true
}

// 右出队列
func (this *queue) Pop() string {
	this.Lock()
	defer this.Unlock()
	if len(this.items) == 0 {
		return ""
	}
	endIndex := len(this.items) - 1
	rtn := this.items[endIndex]
	this.items = this.items[:endIndex]
	return rtn
}

// 左出队列
func (this *queue) Shift() string {
	this.Lock()
	defer this.Unlock()
	if len(this.items) == 0 {
		return ""
	}
	rtn := this.items[0]
	if len(this.items) > 1 {
		this.items = this.items[1:]
	} else {
		this.items = []string{}
	}
	return rtn
}

// 队列长度
func (this *queue) length() int {
	this.Lock()
	defer this.Unlock()
	return len(this.items)
}
