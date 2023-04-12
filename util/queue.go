package util

import "sync"

// 队列
type Queue struct {
	// 互斥锁
	sync.Mutex
	Items []string
}

// Push 右入队列
func (this *Queue) Push(items ...string) bool {
	this.Lock()
	defer this.Unlock()
	this.Items = append(this.Items, items...)
	return true
}

// Unshift 左入队列
func (this *Queue) Unshift(items ...string) bool {
	this.Lock()
	defer this.Unlock()
	readyItems := make([]string, len(items))
	this.Items = append(readyItems, this.Items...)
	return true
}

// 右出队列
func (this *Queue) Pop() string {
	this.Lock()
	defer this.Unlock()
	if len(this.Items) == 0 {
		return ""
	}
	endIndex := len(this.Items) - 1
	rtn := this.Items[endIndex]
	this.Items = this.Items[:endIndex]
	return rtn
}

// 左出队列
func (this *Queue) Shift() string {
	this.Lock()
	defer this.Unlock()
	if len(this.Items) == 0 {
		return ""
	}
	rtn := this.Items[0]
	if len(this.Items) > 1 {
		this.Items = this.Items[1:]
	} else {
		this.Items = []string{}
	}
	return rtn
}

// 队列长度
func (this *Queue) Length() int {
	this.Lock()
	defer this.Unlock()
	return len(this.Items)
}
