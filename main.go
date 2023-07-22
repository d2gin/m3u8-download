package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"m3u8-download/util"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	_url       = flag.String("url", "", "m3u8 url.")
	_file      = flag.String("file", "", "m3u8 file path.")
	_host      = flag.String("host", "", "host prefix.")
	_co        = flag.String("co", "", "goroutine total.")
	_output    = flag.String("output", "", "output dir.")
	_proxy     = flag.String("proxy", "", "proxy host")
	dataDir    = "output/"
	wg         = &sync.WaitGroup{}
	urlQueue   *util.Queue
	total      = 0
	complete   = 0
	fileHeader util.M3U8Header
)

func main() {
	flag.Parse()
	var (
		urlInfo      *url.URL
		indexContent string
		pureContent  string
		saveDir      string
	)
	util.G.Proxy = *_proxy
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

	if len(*_url) > 0 {
		urlInfo, _ = url.Parse(*_url)
		req := &util.Request{
			Header: map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
			},
			Proxy: util.G.Proxy,
		}
		resp, err := req.Do(*_url)
		if err != nil {
			panic(err)
		}
		body, _ := io.ReadAll(resp.Body)
		indexContent = string(body)
		fileHeader = util.ParseM3U8File(indexContent, *_url)
	} else if util.PathExists(*_file) {
		urlInfo, _ = url.Parse(*_host)
		if urlInfo.Host == "" {
			// panic("file host empty")
		}
		_bytes, err := os.ReadFile(*_file)
		if err != nil {
			panic(err)
		}
		indexContent = string(_bytes)
		fileHeader = util.ParseM3U8File(indexContent, *_host)
	} else {
		panic("Invalid data")
	}
	saveDir = strings.TrimSuffix(dataDir, "/") + "/" + func() string {
		d := urlInfo.String()
		d = strings.ReplaceAll(d, "/", "_")
		d = strings.ReplaceAll(d, `\`, "_")
		d = strings.ReplaceAll(d, "&", "_")
		d = strings.ReplaceAll(d, " ", "_")
		d = strings.ReplaceAll(d, ":", "_")
		d = strings.ReplaceAll(d, "?", "_")
		d = strings.ReplaceAll(d, "*", "_")
		d = strings.ReplaceAll(d, "|", "_")
		d = strings.ReplaceAll(d, "<", "_")
		d = strings.ReplaceAll(d, ">", "_")
		return d
	}()
	// 创建文件夹
	for _, d := range []string{dataDir, saveDir} {
		if !util.PathExists(d) {
			err := os.MkdirAll(d, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
	}
	flagMatch, _ := regexp.MatchString("^#EXTM3U", indexContent)
	if !flagMatch {
		panic("Invalid data")
	}
	pureContent = indexContent
	// 删除注释
	pureContent = regexp.MustCompile("#.+\n*").ReplaceAllString(pureContent, "")
	pureContent = strings.TrimSpace(pureContent)
	// 每行一个文件
	lines := strings.Split(pureContent, "\n")
	// 统计数量
	total = len(lines)
	fmt.Println("EXT-X-VERSION: " + strconv.Itoa(fileHeader.Version))
	fmt.Println("EXT-X-TARGETDURATION: " + strconv.Itoa(fileHeader.TargetDuration))
	fmt.Println("EXT-X-PLAYLIST-TYPE: " + fileHeader.PlaylistType)
	fmt.Println("Encrypted: " + strconv.FormatBool(fileHeader.Encrypted) + func() string {
		if fileHeader.Encrypted {
			return ", " + fileHeader.Encryption.Method
		}
		return ""
	}())
	// 队列，让所有协程竞争这个队列数据
	urlQueue = &util.Queue{
		Items: lines,
	}
	// 创建协程
	for i := 1; i <= coTotal; i++ {
		wg.Add(1)
		go saveProc(saveDir, *urlInfo)
	}
	// 阻塞主线程，等所有协程执行完再往下执行
	wg.Wait()
	fmt.Println("")
	fmt.Println(">", "All goroutine done")
	// 生成ffmpeg文件索引文件
	mergeTxt := ""
	// @todo 按照习惯，这里使用`lines`枚举更合理，但是这样会出现序列错乱问题。
	for _, line := range strings.Split(pureContent, "\n") {
		filename := util.Basename(line)
		mergeTxt += "file '" + filename + "'\n"
	}
	os.WriteFile(saveDir+"/merge.txt", []byte(mergeTxt), fs.ModePerm)
	fmt.Println("")
	fmt.Println(">", "Run the following command to generate an mp4 file:")
	fmt.Println("ffmpeg -f concat -i " + strings.TrimSuffix(saveDir, "/") + "/merge.txt -c copy output-" + strconv.Itoa(int(time.Now().Unix())) + ".mp4")
}

func mergeScript(indexContent string, saveDir string) {
	mergeTxt := ""
	// 删除注释
	indexContent = regexp.MustCompile("#.+\n*").ReplaceAllString(indexContent, "")
	indexContent = strings.TrimSpace(indexContent)
	for _, line := range strings.Split(indexContent, "\n") {
		filename := util.Basename(line)
		mergeTxt += "file '" + filename + "'\n"
	}
	os.WriteFile(saveDir+"/merge.txt", []byte(mergeTxt), fs.ModePerm)
}

// 协程函数
func saveProc(saveDir string, urlInfo url.URL) {
	//id := rand.Intn(999999)
	fmt.Printf("\rProgress: %d / %d", complete, total)
	for urlQueue.Length() > 0 {
		// 让每个协程每次下载一个ts文件
		tsName := strings.TrimSpace(urlQueue.Pop())
		if tsName == "" {
			continue
		}
		tsUrl := util.UrlUnparse(tsName, urlInfo)
		filename := util.Basename(tsName)
		result, _ := util.SaveTsFile(tsUrl, saveDir+"/"+filename)
		//fmt.Println(">", id, tsUrl, result)
		if !result {
			urlQueue.Push(tsName)
		} else {
			complete++
			if fileHeader.Encrypted {
				if content, err := os.ReadFile(saveDir + "/" + filename); err == nil {
					// 解码
					if strings.ToLower(fileHeader.Encryption.Method) == "aes-128" {
						if content, err = util.DecryptAES128(content, []byte(fileHeader.Encryption.Key), []byte(fileHeader.Encryption.IV)); err == nil {
							os.WriteFile(saveDir+"/"+filename, content, fs.ModePerm)
						}
					}
					// 其他解码
				}
			}
		}
		fmt.Printf("\rProgress: %d / %d", complete, total)
	}
	//fmt.Println(id, "complete")
	wg.Done()
}
