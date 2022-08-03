package mesos

import (
	"net"
	"strconv"
	"time"

	mesosutil "github.com/AVENTER-UG/mesos-util"
	mesosproto "github.com/AVENTER-UG/mesos-util/proto"

	"github.com/sirupsen/logrus"
)

// StartDatastore is starting the datastore container
func (e *Scheduler) StartDatastore(taskID string) {
	if e.API.CountRedisKey(e.Framework.FrameworkName+":datastore:*") >= e.Config.DSMax {
		return
	}

	cmd := e.defaultCommand(taskID)

	cmd.ContainerType = "DOCKER"
	cmd.Privileged = false
	cmd.Memory = e.Config.DSMEM
	cmd.CPU = e.Config.DSCPU
	cmd.Disk = e.Config.DSDISK
	cmd.TaskName = e.Framework.FrameworkName + ":datastore"
	cmd.Hostname = e.Framework.FrameworkName + "datastore" + e.Config.Domain
	cmd.DockerParameter = e.addDockerParameter(make([]mesosproto.Parameter, 0), mesosproto.Parameter{Key: "cap-add", Value: "NET_ADMIN"})
	cmd.Instances = e.Config.DSMax
	cmd.Shell = false

	// if mesos cni is unset, then use docker cni
	if e.Framework.MesosCNI == "" {
		// net-alias is only supported onuser-defined networks
		if e.Config.DockerCNI != "bridge" {
			cmd.DockerParameter = e.addDockerParameter(cmd.DockerParameter, mesosproto.Parameter{Key: "net-alias", Value: e.Framework.FrameworkName + "datastore"})
		}
	}

	// if we use etcd as datastore
	if e.Config.DSEtcd {
		cmd = e.setETCD(cmd)
	}

	// if we use mysql/maraidb as datastore
	if e.Config.DSMySQL {
		cmd = e.setMySQL(cmd)
	}

	// get free hostport. If there is no one, do not start
	hostport := e.getRandomHostPort(1)
	if hostport == 0 {
		logrus.WithField("func", "StartDatastore").Error("Could not find free ports")
		return
	}
	protocol := "tcp"
	containerPort, _ := strconv.ParseUint(e.Config.DSPort, 10, 32)
	cmd.DockerPortMappings = []mesosproto.ContainerInfo_DockerInfo_PortMapping{
		{
			HostPort:      hostport,
			ContainerPort: uint32(containerPort),
			Protocol:      &protocol,
		},
	}

	cmd.Discovery = mesosproto.DiscoveryInfo{
		Visibility: 2,
		Name:       &cmd.TaskName,
		Ports: &mesosproto.Ports{
			Ports: []mesosproto.Port{
				{
					Number:   cmd.DockerPortMappings[0].HostPort,
					Name:     func() *string { x := "datastore"; return &x }(),
					Protocol: cmd.DockerPortMappings[0].Protocol,
				},
			},
		},
	}

	// store mesos task in DB
	logrus.WithField("func", "StartDatastore").Info("Schedule Datastore")
	e.API.SaveTaskRedis(cmd)
}

// healthCheckETCD check the health of all etcdservers. Return true if all are fine.
func (e *Scheduler) healthCheckDatastore() bool {
	// Hold the at all state of the datastore service.
	dsState := false

	keys := e.API.GetAllRedisKeys(e.Framework.FrameworkName + ":datastore:*")
	for keys.Next(e.API.Redis.RedisCTX) {
		key := e.API.GetRedisKey(keys.Val())
		task := mesosutil.DecodeTask(key)

		if task.State == "TASK_RUNNING" && len(task.NetworkInfo) > 0 {
			if e.connectPort(task.MesosAgent.Slaves[0].Hostname, task.DockerPortMappings[0].GetHostPort()) {
				dsState = true
			}
		}
	}

	logrus.WithField("func", "healthCheckDatastore").Debug("Datastore Health: ", dsState)
	return dsState
}

// check if the remote port is listening
func (e *Scheduler) connectPort(host string, port uint32) bool {
	timeout := 5 * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.FormatUint(uint64(port), 10)), timeout)
	if err != nil {
		logrus.WithField("func", "connectPort").Debug("Hostname: ", host)
		logrus.WithField("func", "connectPort").Debug("Port: ", strconv.FormatUint(uint64(port), 10))
		logrus.WithField("func", "connectPort").Debug("Error: ", err.Error())
		return false
	}
	if conn != nil {
		defer conn.Close()
		return true
	}

	return false
}

// set mysql parameter of the mesos task
func (e *Scheduler) setMySQL(cmd mesosutil.Command) mesosutil.Command {
	cmd.ContainerImage = e.Config.ImageMySQL
	cmd.Environment.Variables = []mesosproto.Environment_Variable{
		{
			Name:  "SERVICE_NAME",
			Value: &cmd.TaskName,
		},
		{
			Name: "MYSQL_ROOT_PASSWORD",
			Value: func() *string {
				x := e.Config.DSMySQLPassword
				return &x
			}(),
		},
		{
			Name: "MYSQL_DATABASE",
			Value: func() *string {
				x := "k3s"
				return &x
			}(),
		},
	}
	return cmd
}

// set etcd parameter of the mesos task
func (e *Scheduler) setETCD(cmd mesosutil.Command) mesosutil.Command {
	cmd.ContainerImage = e.Config.ImageETCD
	cmd.Command = "/opt/bitnami/etcd/bin/etcd --listen-client-urls http://0.0.0.0:" + e.Config.DSPort + " --election-timeout '50000' --heartbeat-interval '5000'"
	AdvertiseURL := "http://" + cmd.Hostname + ":" + e.Config.DSPort

	AllowNoneAuthentication := "yes"

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
	return cmd
}
