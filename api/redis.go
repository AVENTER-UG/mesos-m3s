package api

import (
	"context"
	"encoding/json"

	mesosutil "github.com/AVENTER-UG/mesos-util"
	mesosproto "github.com/AVENTER-UG/mesos-util/proto"
	goredis "github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// Redis struct about the redis connection
type Redis struct {
	RedisClient *goredis.Client
	RedisCTX    context.Context
}

// GetAllRedisKeys get out all keys in redis depends to the pattern
func (e *API)GetAllRedisKeys(pattern string) *goredis.ScanIterator {
	val := e.Redis.RedisClient.Scan(e.Redis.RedisCTX, 0, pattern, 0).Iterator()
	if err := val.Err(); err != nil {
		logrus.Error("getAllRedisKeys: ", err)
	}
	return val
}

// GetRedisKey get out all values to a key
func (e *API)GetRedisKey(key string) string {
	val, err := e.Redis.RedisClient.Get(e.Redis.RedisCTX, key).Result()
	if err != nil {
		logrus.Error("getRedisKey: ", err)
	}
	return val
}

// DelRedisKey will delete a redis key
func (e *API)DelRedisKey(key string) int64 {
	val, err := e.Redis.RedisClient.Del(e.Redis.RedisCTX, key).Result()
	if err != nil {
		logrus.Error("delRedisKey: ", err)
	}

	return val
}

// GetTaskFromEvent get out the task to an event
func (e *API)GetTaskFromEvent(update *mesosproto.Event_Update) mesosutil.Command {
	// search matched taskid in redis and update the status
	keys := e.GetAllRedisKeys("*")
	for keys.Next(e.Redis.RedisCTX) {
		// get the values of the current key
		key := e.GetRedisKey(keys.Val())

		// update the status of the matches task
		var task mesosutil.Command
		json.Unmarshal([]byte(key), &task)
		if task.TaskID == update.Status.TaskID.Value {
			task.State = update.Status.State.String()

			return task
		}
	}
	return mesosutil.Command{}
}

// CountRedisKey will get back the count of the redis key
func (e *API)CountRedisKey(pattern string) int {
	keys := e.GetAllRedisKeys(pattern)
	count := 0
	for keys.Next(e.Redis.RedisCTX) {
		count++
	}
	logrus.Debug("CountRedisKey: ", pattern, count)
	return count
}

// SaveConfig store the current framework config
func (e *API)SaveConfig() {
	data, _ := json.Marshal(e.Config)
	err := e.Redis.RedisClient.Set(e.Redis.RedisCTX, e.Framework.FrameworkName+":framework_config", data, 0).Err()
	if err != nil {
		logrus.Error("Framework save config and state into redis Error: ", err)
	}
}

// PingRedis to check the health of redis
func (e *API)PingRedis() error {
	pong, err := e.Redis.RedisClient.Ping(e.Redis.RedisCTX).Result()
	logrus.Debug("Redis Health: ", pong, err)
	if err != nil {
		return err
	}
	return nil
}

// ConnectRedis will connect the redis DB and save the client pointer
func (e *API)ConnectRedis() {
	var redisOptions goredis.Options
	redisOptions.Addr = e.Config.RedisServer
	redisOptions.DB = e.Config.RedisDB
	if e.Config.RedisPassword != "" {
		redisOptions.Password = e.Config.RedisPassword
	}

	e.Redis.RedisClient = goredis.NewClient(&redisOptions)
	e.Redis.RedisCTX = context.Background()

	err := e.PingRedis()
	if err != nil {
		e.ConnectRedis()
	}
}
