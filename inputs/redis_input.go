package inputs

import (
	"encoding/json"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
	"github.com/yankeguo/xlog"
)

// Redis a redis client wrapper for filebeat events
type Redis struct {
	Client *redis.Client
	Key    string // LIST key for BLPOP
}

// DialRedis dial a redis and create *Redis
func DialRedis(url string, key string) (b *Redis, err error) {
	var opt *redis.Options
	// parse redis.Options
	if opt, err = redis.ParseURL(url); err != nil {
		return
	}
	// create redis client and ping
	var client = redis.NewClient(opt)
	if err = client.Ping().Err(); err != nil {
		return
	}
	b = &Redis{Client: client, Key: key}
	return
}

// Close close the underlying redis client
func (b *Redis) Close() error {
	return b.Client.Close()
}

// Next fetch next event, nil for timeout, or JSON unmarshal error
func (b *Redis) Next() (r xlog.RecordConvertible, err error) {
	var ret []string
	// BLPOP
	if ret, err = b.Client.BLPop(time.Second*3, b.Key).Result(); err != nil {
		// redis.Nil should be ignored
		if err == redis.Nil {
			err = nil
		}
		return
	}
	// length == 0 for timeout, should be ignored
	if len(ret) == 0 {
		return
	}
	// length 1 for single key, 2 for multiple key, so ret[-1] should be fine
	raw := ret[len(ret)-1]
	// unmarshal json
	var be BeatEvent
	if err = json.Unmarshal([]byte(raw), &be); err != nil {
		// JSON unmarshal error should be ignored
		err = nil
		return
	}
	r = be
	return
}

// Recover requeue a beat entry with RPUSH
func (b *Redis) Recover(r xlog.RecordConvertible) (err error) {
	// check if it's a BeatEvent
	var be BeatEvent
	var ok bool
	if be, ok = r.(BeatEvent); !ok {
		err = errors.New("not a BeatEvent")
		return
	}
	// marshal JSON
	var buf []byte
	if buf, err = json.Marshal(be); err != nil {
		return
	}
	// RPUSH
	_, err = b.Client.RPush(b.Key, string(buf)).Result()
	return
}
