package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

// Init 建立 Redis 连接(拆分字段)。
func Init(addr, password string, db int) error {
	RDB = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return ping()
}

// InitURL 用连接串建立 Redis 连接(redis://:pwd@host:port/db)。
func InitURL(url string) error {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return err
	}
	RDB = redis.NewClient(opt)
	return ping()
}

func ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return RDB.Ping(ctx).Err()
}
