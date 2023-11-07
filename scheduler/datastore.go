package scheduler

import (
	"net"
	"strconv"
	"time"

	mesosproto "github.com/AVENTER-UG/mesos-m3s/proto"
	cfg "github.com/AVENTER-UG/mesos-m3s/types"

	"github.com/sirupsen/logrus"
)

// StartDatastore is starting the datastore container
func (e *Scheduler) StartDatastore(taskID string) {
	if e.Redis.CountRedisKey(e.Framework.FrameworkName+":datastore:*", "") >= e.Config.DSMax {
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
		e.setETCD(&cmd)
	}

	// if we use mysql/maraidb as datastore
	if e.Config.DSMySQL {
		e.setMySQL(&cmd)
	}

	protocol := "tcp"
	containerPort, _ := strconv.ParseUint(e.Config.DSPort, 10, 32)
	cmd.DockerPortMappings = []mesosproto.ContainerInfo_DockerInfo_PortMapping{
		{
			HostPort:      0,
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
	e.Redis.SaveTaskRedis(cmd)
}

// healthCheckDatastore check the health of all datastore ervers. Return true if all are fine.
func (e *Scheduler) healthCheckDatastore() bool {
	// Hold the at all state of the datastore service.
	dsState := false

	keys := e.Redis.GetAllRedisKeys(e.Framework.FrameworkName + ":datastore:*")
	for keys.Next(e.Redis.CTX) {
		key := e.Redis.GetRedisKey(keys.Val())
		task := e.Mesos.DecodeTask(key)
		if task.State == "TASK_RUNNING" && len(task.NetworkInfo) > 0 {
			// if the framework is running as container, and the task hostname is the same like the frameworks one,
			// then use the containerport instead of the random hostport
			if e.Config.DockerRunning && (task.MesosAgent.Hostname == e.Config.Hostname) {
				if e.connectPort(task.Hostname, task.DockerPortMappings[0].GetContainerPort()) {
					dsState = true
				}
			} else {
				if e.connectPort(task.MesosAgent.Hostname, task.DockerPortMappings[0].GetHostPort()) {
					dsState = true
				}
			}
		}
	}

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
func (e *Scheduler) setMySQL(cmd *cfg.Command) {
	cmd.ContainerImage = e.Config.ImageMySQL
	//cmd.Shell = true
	// Enable TLS for Mariadb
	if e.Config.DSMySQLSSL {
		cmd.Arguments = e.appendString(make([]string, 0), "--ssl-ca=/var/lib/mysql/ca.pem")
		cmd.Arguments = e.appendString(cmd.Arguments, "--ssl-cert=/var/lib/mysql/server-cert.pem")
		cmd.Arguments = e.appendString(cmd.Arguments, "--ssl-key=/var/lib/mysql/server-key.pem")
	}
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
		{
			Name:  "TZ",
			Value: &e.Config.TimeZone,
		},
	}
	cmd.Volumes = []mesosproto.Volume{
		{
			ContainerPath: "/var/lib/mysql",
			Mode:          mesosproto.RW.Enum(),
			Source: &mesosproto.Volume_Source{
				Type: mesosproto.Volume_Source_DOCKER_VOLUME,
				DockerVolume: &mesosproto.Volume_Source_DockerVolume{
					Driver: &e.Config.VolumeDriver,
					Name:   e.Config.VolumeDS,
				},
			},
		},
	}
}

// set etcd parameter of the mesos task
func (e *Scheduler) setETCD(cmd *cfg.Command) {
	cmd.ContainerImage = e.Config.ImageETCD
	cmd.Command = "/usr/local/bin/etcd --listen-client-urls http://0.0.0.0:" + e.Config.DSPort + " --election-timeout '50000' --heartbeat-interval '5000'"
	cmd.Shell = true
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
	}
	cmd.Volumes = []mesosproto.Volume{
		{
			ContainerPath: "/default.etcd",
			Mode:          mesosproto.RW.Enum(),
			Source: &mesosproto.Volume_Source{
				Type: mesosproto.Volume_Source_DOCKER_VOLUME,
				DockerVolume: &mesosproto.Volume_Source_DockerVolume{
					Driver: &e.Config.VolumeDriver,
					Name:   e.Config.VolumeDS,
				},
			},
		},
	}
}
