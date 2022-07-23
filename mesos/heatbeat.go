package mesos

import (
	"encoding/json"
	"time"

	mesosutil "github.com/AVENTER-UG/mesos-util"
	"github.com/sirupsen/logrus"
)

// Heartbeat function for mesos
func (e *Scheduler)Heartbeat() {
	// Check Connection state of Redis
	err := e.API.PingRedis()
	if err != nil {
		e.API.ConnectRedis()
	}

	e.K3SHeartbeat()
	keys := e.API.GetAllRedisKeys(e.Framework.FrameworkName + ":*")
	suppress := true

	for keys.Next(e.API.Redis.RedisCTX) {
		// get the values of the current key
		key := e.API.GetRedisKey(keys.Val())

		var task mesosutil.Command
		json.Unmarshal([]byte(key), &task)

		if task.TaskID == "" || task.TaskName == "" {
			continue
		}

		if task.State == "" {
			mesosutil.Revive()
			task.State = "__NEW"
			// these will save the current time at the task. we need it to check
			// if the state will change in the next 'n min. if not, we have to
			// give these task a recall.
			task.StateTime = time.Now()

			// add task to communication channel
			e.Framework.CommandChan <- task

			data, _ := json.Marshal(task)
			err := e.API.Redis.RedisClient.Set(e.API.Redis.RedisCTX, task.TaskName+":"+task.TaskID, data, 0).Err()
			if err != nil {
				logrus.Error("HandleUpdate Redis set Error: ", err)
			}

			logrus.Info("Scheduled Mesos Task: ", task.TaskName)
		}

		if task.State == "__NEW" {
			suppress = false
			e.Config.Suppress = false
		}
	}

	if suppress && !e.Config.Suppress {
		mesosutil.SuppressFramework()
		e.Config.Suppress = true
	}
}

// K3SHeartbeat to execute K3S Bootstrap API Server commands
func (e *Scheduler)K3SHeartbeat() {
	if e.API.CountRedisKey(e.Framework.FrameworkName+":etcd:*") < e.Config.ETCDMax {
		e.StartEtcd("")
	}
	if e.getEtcdStatus() == "TASK_RUNNING" && !e.IsK3SServerRunning() {
		if e.API.CountRedisKey(e.Framework.FrameworkName+":server:*") < e.Config.K3SServerMax {
			e.StartK3SServer("")
		}
	}
	if e.IsK3SServerRunning() {
		if e.API.CountRedisKey(e.Framework.FrameworkName+":agent:*") < e.Config.K3SAgentMax {
			e.StartK3SAgent("")
		}
	}
}

// HeartbeatLoop - The main loop for the hearbeat
func (e *Scheduler) HeartbeatLoop() {
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()
	for ; true; <-ticker.C {
		e.Heartbeat()
	}
}

