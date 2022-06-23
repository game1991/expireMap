package expiremap

import "errors"

var (
	// ErrNotInit 缓存未初始化
	ErrNotInit = errors.New("未初始化")
	// ErrTooLoogKey key 太长
	ErrTooLoogKey = errors.New("key长度过长,不应超过1024字节")
)
