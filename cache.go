package expiremap

import (
	"container/list"
	"sync"
	"time"
)

const (
	// DEFAULTEXPIRED 默认过期时间
	DEFAULTEXPIRED = 24 * time.Hour
	maxDeletion    = 1 << 20
	copyThreshold  = 1 << 10 // 复制old临界点
	// MaxKeyLenth 默认key长度
	MaxKeyLenth = 1024
)

// Cache 缓存对象
type Cache struct {
	MaxEntries int //最大缓存项
	bucket     *Container
	pool       *sync.Pool // 对象池
	mutex      *sync.RWMutex
}

// Container 缓存容器
type Container struct {
	deleteOld int
	deleteNew int
	oldbucket map[string]*list.Element
	newbucket map[string]*list.Element
	lruList   *list.List // 节点链表结果
}

// CacheEntry 一个单独的缓存对象
type CacheEntry struct {
	Key        string
	Value      interface{}
	Expiration int64 //(时间戳精度到秒)
}

func (c *Cache) checkInit() error {
	if c.bucket == nil || c.mutex == nil || c.pool == nil {
		return ErrNotInit
	}
	return nil
}

func (c *Cache) Set(key string, value interface{}, timestamp time.Duration) error {
	if err := c.checkInit(); err != nil {
		return err
	}

	if len(key) > MaxKeyLenth {
		return ErrTooLoogKey
	}
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

	return nil
}

func (c *Cache) Get(key string, timestamp time.Duration) (interface{}, error) {
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

func (c *Cache) get(key string) bool {

}

//
func (c *Cache) isExpired(key string) bool {

}

func (c *Cache) Delete(key string) error {
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

func (c *cache) delete(key string) error {

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
		if ttl < 0 {
			// 处理过期的key
			c.Delete(key)
			return 0
		}
		return ttl
	}
	return 0
}

func (c *Cache) Size() int {
	c.mutex.RLock()
	size := len(c.old) + len(c.new)
	c.mutex.RUnlock()
	return size
}

// NewCache 获取一个内存缓存对象并设置默认参数
//默认每个储存容器初始化大小为5W
//@maxEntries 缓存项储存最大值，超过该值会被移除缓存
//@capacity 初始缓存容量
func NewCache(maxEntries int, capacity int) *Cache {
	return &Cache{
		MaxEntries: maxEntries,
		bucket: &Container{
			oldbucket: make(map[string]*list.Element, capacity),
			newbucket: make(map[string]*list.Element, capacity),
			lruList:   list.New(),
		},
		pool: &sync.Pool{
			New: func() interface{} {
				return &CacheEntry{}
			},
		},
		mutex: new(sync.RWMutex),
	}
}
