package mesos

import (
	"encoding/json"

	mesosutil "github.com/AVENTER-UG/mesos-util"
	mesosproto "github.com/AVENTER-UG/mesos-util/proto"
	"github.com/AVENTER-UG/util"

	"github.com/sirupsen/logrus"
)

func (e *Scheduler)getEtcdStatus() string {
	keys := e.API.GetAllRedisKeys(e.Framework.FrameworkName + ":etcd:*")

	for keys.Next(e.API.Redis.RedisCTX) {
		key := e.API.GetRedisKey(keys.Val())
		var task mesosutil.Command
		json.Unmarshal([]byte(key), &task)
		return task.State
	}
	return ""
}

// StartEtcd is starting the etcd
func (e *Scheduler)StartEtcd(taskID string) {
	var cmd mesosutil.Command

	// if taskID is 0, then its a new task and we have to create a new ID
	newTaskID := taskID
	if taskID == "" {
		newTaskID, _ = util.GenUUID()
	}

	cni := e.Framework.MesosCNI

	cmd.TaskID = newTaskID
	cmd.ContainerType = "DOCKER"
	cmd.ContainerImage = e.Config.ImageETCD
	cmd.Shell = true
	cmd.Privileged = false
	cmd.Memory = e.Config.ETCDMEM
	cmd.CPU = e.Config.ETCDCPU
	cmd.Disk = e.Config.ETCDDISK
	cmd.TaskName = e.Framework.FrameworkName + ":etcd"
	cmd.Hostname = e.Framework.FrameworkName + "etcd" + e.Config.Domain
	cmd.DockerParameter = e.addDockerParameter(make([]mesosproto.Parameter, 0), mesosproto.Parameter{Key: "cap-add", Value: "NET_ADMIN"})
	// if mesos cni is unset, then use docker cni
	if e.Framework.MesosCNI == "" {
		// net-alias is only supported onuser-defined networks
		if e.Config.DockerCNI != "bridge" {
			cmd.NetworkMode = "user"
			cni = e.Config.DockerCNI
			cmd.DockerParameter = e.addDockerParameter(cmd.DockerParameter, mesosproto.Parameter{Key: "net-alias", Value: e.Framework.FrameworkName + "etcd"})
		}
	}

	cmd.NetworkInfo = []mesosproto.NetworkInfo{{
		Name: &cni,
	}}

	AllowNoneAuthentication := "yes"
	AdvertiseURL := "http://" + cmd.Hostname + ":2379"

	cmd.Command = "/opt/bitnami/etcd/bin/etcd --listen-client-urls http://0.0.0.0:2379 --election-timeout '50000' --heartbeat-interval '5000'"

	cmd.Environment.Variables = []mesosproto.Environment_Variable{
		{
			Name:  "SERVICE_NAME",
			Value: &cmd.TaskName,
		},
		{
			Name:  "ALLOW_NONE_AUTHENTICATION",
			Value: &AllowNoneAuthentication,
		},
		{
			Name:  "ETCD_ADVERTISE_CLIENT_URLS",
			Value: &AdvertiseURL,
		},
		{
			Name: "ETCD_DATA_DIR",
			Value: func() *string {
				x := "/tmp"
				return &x
			}(),
		},
	}

	cmd.Discovery = mesosproto.DiscoveryInfo{
		Visibility: 1,
		Name:       &cmd.TaskName,
	}

	// store mesos task in DB
	d, _ := json.Marshal(&cmd)
	logrus.Debug("Scheduled Etcd: ", string(d))
	logrus.Info("Scheduled Etcd")
	err := e.API.Redis.RedisClient.Set(e.API.Redis.RedisCTX, cmd.TaskName+":"+newTaskID, d, 0).Err()
	if err != nil {
		logrus.Error("Cloud not store Mesos Task in Redis: ", err)
	}
}

func (e *Scheduler)addDockerParameter(current []mesosproto.Parameter, newValues mesosproto.Parameter) []mesosproto.Parameter {
	return append(current, newValues)
}
