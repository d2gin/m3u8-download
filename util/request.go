package util

import (
	"net/http"
	"net/url"
)

type Header struct {
}

type Request struct {
	Proxy  string
	Header map[string]string
	Url    string
}

func (r *Request) Do(_url string) (*http.Response, error) {
	var proxy http.RoundTripper
	if r.Proxy != "" {
		up, _ := url.Parse(r.Proxy)
		proxy = &http.Transport{
			Proxy: http.ProxyURL(up),
		}
	}
	client := &http.Client{
		Transport: proxy,
	}
	req, err := http.NewRequest("GET", _url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range r.Header {
		req.Header.Add(key, value)
	}
	resp, err := client.Do(req)
	return resp, err
}
