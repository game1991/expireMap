package expiremap

import (
	"sync"
	"time"
)

// Cacher ...
type Cacher interface {
	Set(key string, value interface{}, timestamp time.Duration) error
	Get(key string, timestamp time.Duration) (interface{}, error)
	Delete(key string) error
	TTL(key string) int64
	Size() int
}

var cacheInstance Cacher

const (
	// DEFAULTEXPIRED 默认过期时间
	DEFAULTEXPIRED = 24 * time.Hour
	maxDeletion    = 1 << 20
	copyThreshold  = 1 << 10 // 复制old临界点
)

type cache struct {
	deleteOld int
	deleteNew int
	old       map[string]*val
	new       map[string]*val
	mutex     *sync.RWMutex
}

type val struct {
	data       interface{}
	expiration int64 //(时间戳精度到秒)
}

func (c *cache) Set(key string, value interface{}, timestamp time.Duration) error {
	// 如果传入了过期时间使用过期时间，如果没有传入，则使用默认过期时间
	if timestamp <= 0 {
		timestamp = DEFAULTEXPIRED
	}
	// 设置键保证数据安全，加锁控制写入
	c.mutex.Lock()

	if c.deleteOld <= maxDeletion {
		// 如果可以在另外一个map中找到key，则删除，确保在整体map中只有一个唯一的key
		if _, ok := c.new[key]; ok {
			delete(c.new, key)
			c.deleteNew++
		}
		c.old[key] = &val{data: value, expiration: time.Now().Add(timestamp).Unix()}
	} else {
		//当删除量达到阈值，进行存储到新map中
		if _, ok := c.old[key]; ok {
			delete(c.old, key)
			c.deleteOld++
		}
		c.new[key] = &val{data: value, expiration: time.Now().Add(timestamp).Unix()}
	}
	c.mutex.Unlock()

	time.AfterFunc(timestamp, func() {
		c.Delete(key)
	})

	return nil
}

func (c *cache) Get(key string, timestamp time.Duration) (interface{}, error) {
	var val *val
	var found bool
	c.mutex.RLock()
	val, found = c.old[key]
	if !found {
		// 在两个map中寻找
		val, found = c.new[key]
	}
	c.mutex.RUnlock()

	if !found {
		// key不存在直接返回nil
		return nil, nil
	}

	// 惰性删除
	if time.Now().Unix() > val.expiration {
		c.Delete(key)
	}

	// 如果传参需要更新
	if timestamp > 0 {
		err := c.Set(key, val, timestamp)
		return val.data, err
	}
	return val.data, nil
}

func (c *cache) Delete(key string) error {
	c.mutex.Lock()
	if _, ok := c.old[key]; ok {
		delete(c.old, key)
		c.deleteOld++
	} else if _, ok := c.new[key]; ok {
		delete(c.new, key)
		c.deleteNew++
	}
	if c.deleteOld >= maxDeletion && len(c.old) < copyThreshold {
		for k, v := range c.old {
			c.new[k] = v
		}
		// 由于之前的delete()，map底层只是标记了tophash=empty，所以当删除key数量达阈值，range这个old的map，进行过滤掉tophash<=emptyOne的key
		c.old = c.new
		c.deleteOld = c.deleteNew
		c.new = make(map[string]*val)
		c.deleteNew = 0
	}

	if c.deleteNew >= maxDeletion && len(c.new) < copyThreshold {
		for k, v := range c.new {
			c.old[k] = v
		}
		c.new = make(map[string]*val)
		c.deleteNew = 0
	}

	c.mutex.Unlock()
	return nil
}

func (c *cache) TTL(key string) int64 {
	var found bool
	var val *val
	c.mutex.RLock()
	val, found = c.old[key]
	if !found {
		val, found = c.new[key]
	}
	c.mutex.RUnlock()

	if found {
		// 单位为秒
		ttl := val.expiration - time.Now().Unix()
		if ttl > 0 {
			return ttl
		}
	}
	return 0
}

func (c *cache) Size() int {
	c.mutex.RLock()
	size := len(c.old) + len(c.new)
	c.mutex.RUnlock()
	return size
}

func newCache() *cache {
	return &cache{
		old:   make(map[string]*val),
		new:   make(map[string]*val),
		mutex: new(sync.RWMutex),
	}
}

// GetCacher 获取接口实例
func GetCacher() Cacher {
	if cacheInstance == nil {
		cacheInstance = newCache()
	}
	return cacheInstance
}
