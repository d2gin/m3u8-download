package util

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
)

// UrlUnparse url合成
func UrlUnparse(filename string, urlInfo url.URL) string {
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

// PathExists 判断路径是否存在
func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return false
}

// SaveTsFile 保存文件
func SaveTsFile(url string, filepath string) (bool, error) {
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
	file, err := os.Create(filepath)
	if err != nil {
		return false, err
	}
	n, err := file.Write(buffer)
	if err != nil || n <= 0 {
		return false, errors.New("write fail")
	}
	return true, nil
}
