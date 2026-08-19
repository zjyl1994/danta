package storage

import "time"

// ObjectInfo ListObjectsV2 返回的对象元数据
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Storage 对象存储接口
type Storage interface {
	// Put 写对象（带 Content-Type 与 Cache-Control）
	Put(key string, data []byte, contentType string, cacheMaxAge int) error
	// Delete 批量删除
	Delete(keys []string) error
	// ListAll 分页扫全桶，逐项回调
	ListAll(fn func(ObjectInfo) error) error
	// ListPrefix 带 prefix 的全桶扫描
	ListPrefix(prefix string, fn func(ObjectInfo) error) error
}
