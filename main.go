package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"m3u8-download/util"
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
	urlQueue *util.Queue
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

	if !util.PathExists(dataDir) {
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
	} else if util.PathExists(*_file) {
		urlInfo, _ = url.Parse(*_host)
		if urlInfo.Host == "" {
			// panic("file host empty")
		}
		_bytes, err := os.ReadFile(*_file)
		if err != nil {
			panic(err)
		}
		tsList = string(_bytes)
	} else {
		panic("Invalid data")
	}
	flagMatch, _ := regexp.MatchString("^#EXTM3U", tsList)
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
	urlQueue = &util.Queue{
		Items: lines,
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
	// @todo 按照习惯，这里使用`lines`枚举更合理，但是这样会出现序列错乱问题。
	for _, line := range strings.Split(tsList, "\n") {
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
	for urlQueue.Length() > 0 {
		tsName := strings.TrimSpace(urlQueue.Pop()) // 让每个协程每次下载一个ts文件
		if tsName == "" {
			return
		}
		tsUrl := util.UrlUnparse(tsName, urlInfo)
		filename := path.Base(tsName)
		result, _ := util.SaveTsFile(tsUrl, dataDir+"/"+filename)
		fmt.Println(">", id, tsUrl, result)
		if !result {
			urlQueue.Push(tsName)
		}
	}
	//fmt.Println(id, "complete")
	wg.Done()
}
