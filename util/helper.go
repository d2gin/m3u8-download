package util

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Encryption struct {
	Method string
	Key    string
	IV     string
}

type M3U8Header struct {
	Version        int
	TargetDuration int
	PlaylistType   string
	Encrypted      bool
	Encryption     Encryption
}

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
	if PathExists(filepath) {
		return true, nil
	}
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

func ParseM3U8File(content string, indexUrl string) M3U8Header {
	header := M3U8Header{}
	var resp *http.Response
	var err error
	var bf []byte
	var ivBytes []byte
	if content == "" {
		if resp, err = http.Get(indexUrl); err != nil {
			panic("文件获取失败")
		}
		buf, _ := io.ReadAll(resp.Body)
		content = string(buf)
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXT-X-VERSION") {
			version := strings.TrimPrefix(line, "#EXT-X-VERSION:")
			header.Version, _ = strconv.Atoi(version)
		} else if strings.HasPrefix(line, "#EXT-X-KEY") {
			text := strings.TrimPrefix(line, "#EXT-X-KEY:")
			text = strings.ReplaceAll(text, ",", "&")
			queryinfo, _ := url.Parse("?" + text)
			uri := strings.Trim(queryinfo.Query().Get("URI"), `"`)
			indexUrlInfo, _ := url.Parse(indexUrl)
			keyurl := UrlUnparse(uri, *indexUrlInfo)
			if resp, err = http.Get(keyurl); err != nil {
				panic("密钥获取失败")
			} else if bf, err = io.ReadAll(resp.Body); err != nil {
				panic("密钥获取失败")
			}
			iv := queryinfo.Query().Get("IV")
			if strings.HasPrefix(iv, "0x") {
				if ivBytes, err = hex.DecodeString(iv[2:]); err != nil {
					panic("iv解码失败")
				}
				iv = string(ivBytes)
			}
			header.Encrypted = true
			header.Encryption = Encryption{
				Method: queryinfo.Query().Get("METHOD"),
				Key:    string(bf),
				IV:     iv,
			}
		} else if strings.HasPrefix(line, "#EXT-X-TARGETDURATION") {
			d := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:")
			header.TargetDuration, _ = strconv.Atoi(d)
		} else if strings.HasPrefix(line, "#EXT-X-PLAYLIST-TYPE") {
			header.PlaylistType = strings.TrimPrefix(line, "#EXT-X-PLAYLIST-TYPE:")
		}
	}
	return header
}

func DecryptAES128(data []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decryptedData := make([]byte, len(data))
	mode.CryptBlocks(decryptedData, data)

	return decryptedData, nil
}
