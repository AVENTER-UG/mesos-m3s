package mesos

import (
	"encoding/json"

	mesosutil "github.com/AVENTER-UG/mesos-util"

	mesosproto "github.com/AVENTER-UG/mesos-util/proto"

	"github.com/sirupsen/logrus"
)

// HandleUpdate will handle the offers event of mesos
func (e *Scheduler)HandleUpdate(event *mesosproto.Event) error {
	logrus.Debug("HandleUpdate")

	update := event.Update

	msg := &mesosproto.Call{
		Type: mesosproto.Call_ACKNOWLEDGE,
		Acknowledge: &mesosproto.Call_Acknowledge{
			AgentID: *update.Status.AgentID,
			TaskID:  update.Status.TaskID,
			UUID:    update.Status.UUID,
		},
	}

	// get the task of the current event, change the state
	task := e.API.GetTaskFromEvent(update)
	task.State = update.Status.State.String()
	task.Agent = update.Status.GetAgentID().Value

	if task.TaskID == "" {
		return nil
	}

	logrus.Debug(task.State)

	switch *update.Status.State {
	case mesosproto.TASK_FAILED:
		// restart task
		task.State = ""
	case mesosproto.TASK_KILLED:
		// remove task
		e.API.DelRedisKey(task.TaskName + ":" + task.TaskID)
		return mesosutil.Call(msg)
	case mesosproto.TASK_LOST:
		// restart task
		task.State = ""
	case mesosproto.TASK_ERROR:
		// restart task
		task.State = ""
	}

	// save the new state
	data, _ := json.Marshal(task)
	err := e.API.Redis.RedisClient.Set(e.API.Redis.RedisCTX, task.TaskName+":"+task.TaskID, data, 0).Err()
	if err != nil {
		logrus.Error("HandleUpdate Redis set Error: ", err)
	}

	return mesosutil.Call(msg)
}
