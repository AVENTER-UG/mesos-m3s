package mesos

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	mesosutil "github.com/AVENTER-UG/mesos-util"
	mesosproto "github.com/AVENTER-UG/mesos-util/proto"
	"github.com/AVENTER-UG/util"

	"github.com/sirupsen/logrus"
)

// StartK3SServer Start K3S with the given id
func (e *Scheduler)StartK3SServer(taskID string) {
	// if taskID is 0, then its a new task and we have to create a new ID
	newTaskID := taskID
	if taskID == "" {
		newTaskID, _ = util.GenUUID()
	}

	cni := e.Framework.MesosCNI

	var cmd mesosutil.Command

	cmd.TaskID = newTaskID

	cmd.ContainerType = "DOCKER"
	cmd.ContainerImage = e.Config.ImageK3S
	cmd.Shell = true
	cmd.Privileged = true
	cmd.ContainerImage = e.Config.ImageK3S
	cmd.Memory = e.Config.K3SServerMEM
	cmd.CPU = e.Config.K3SServerCPU
	cmd.TaskName = e.Framework.FrameworkName + ":server"
	cmd.Hostname = e.Framework.FrameworkName + "server" + e.Config.Domain
	cmd.Command = "$MESOS_SANDBOX/bootstrap '" + e.Config.K3SServerString + e.Config.K3SDocker + " --kube-controller-manager-arg='leader-elect=false' --kube-scheduler-arg='leader-elect=false' -tls-san=" + e.Framework.FrameworkName + "server'"
	cmd.DockerParameter = e.addDockerParameter(make([]mesosproto.Parameter, 0), mesosproto.Parameter{Key: "cap-add", Value: "NET_ADMIN"})
	cmd.DockerParameter = e.addDockerParameter(make([]mesosproto.Parameter, 0), mesosproto.Parameter{Key: "cap-add", Value: "SYS_ADMIN"})
	cmd.DockerParameter = e.addDockerParameter(cmd.DockerParameter, mesosproto.Parameter{Key: "shm-size", Value: e.Config.DockerSHMSize})
	// if mesos cni is unset, then use docker cni
	if e.Framework.MesosCNI == "" {
		// net-alias is only supported onuser-defined networks
		if e.Config.DockerCNI != "bridge" {
			cmd.NetworkMode = "user"
			cni = e.Config.DockerCNI
			cmd.DockerParameter = e.addDockerParameter(cmd.DockerParameter, mesosproto.Parameter{Key: "net-alias", Value: e.Framework.FrameworkName + "server"})
		}
	}

	cmd.NetworkInfo = []mesosproto.NetworkInfo{{
		Name: &cni,
	}}

	cmd.Uris = []mesosproto.CommandInfo_URI{
		{
			Value:      e.Config.BootstrapURL,
			Extract:    func() *bool { x := false; return &x }(),
			Executable: func() *bool { x := true; return &x }(),
			Cache:      func() *bool { x := false; return &x }(),
			OutputFile: func() *string { x := "bootstrap"; return &x }(),
		},
	}
	cmd.Volumes = []mesosproto.Volume{
		{
			ContainerPath: "/var/lib/rancher/k3s",
			Mode:          mesosproto.RW.Enum(),
			Source: &mesosproto.Volume_Source{
				Type: mesosproto.Volume_Source_DOCKER_VOLUME,
				DockerVolume: &mesosproto.Volume_Source_DockerVolume{
					Driver: &e.Config.VolumeDriver,
					Name:   e.Config.VolumeK3SServer,
				},
			},
		},
	}

	// get free hostport. If there is no one, do not start
	hostport := e.getRandomHostPort(3)
	if hostport == 0 {
		logrus.WithField("func", "StartK3SServer").Error("Could not find free ports")
		return
	}
	protocol := "tcp"
	cmd.DockerPortMappings = []mesosproto.ContainerInfo_DockerInfo_PortMapping{
		{
			HostPort:      hostport,
			ContainerPort: 10422,
			Protocol:      &protocol,
		},
		{
			HostPort:      hostport + 1,
			ContainerPort: 6443,
			Protocol:      &protocol,
		},
		{
			HostPort:      hostport + 2,
			ContainerPort: 8080,
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
					Name:     func() *string { x := "api"; return &x }(),
					Protocol: cmd.DockerPortMappings[0].Protocol,
				},
				{
					Number:   cmd.DockerPortMappings[1].HostPort,
					Name:     func() *string { x := "kubernetes"; return &x }(),
					Protocol: cmd.DockerPortMappings[1].Protocol,
				},
				{
					Number:   cmd.DockerPortMappings[2].HostPort,
					Name:     func() *string { x := "http"; return &x }(),
					Protocol: cmd.DockerPortMappings[2].Protocol,
				},
			},
		},
	}

	e.CreateK3SServerString()

	cmd.Environment.Variables = []mesosproto.Environment_Variable{
		{
			Name:  "SERVICE_NAME",
			Value: &cmd.TaskName,
		},
		{
			Name:  "K3SFRAMEWORK_TYPE",
			Value: func() *string { x := "server"; return &x }(),
		},
		{
			Name:  "K3S_URL",
			Value: &e.Config.K3SServerURL,
		},
		{
			Name:  "K3S_TOKEN",
			Value: &e.Config.K3SToken,
		},
		{
			Name:  "K3S_KUBECONFIG_OUTPUT",
			Value: func() *string { x := "/mnt/mesos/sandbox/kubee.Config.yaml"; return &x }(),
		},
		{
			Name:  "K3S_KUBECONFIG_MODE",
			Value: func() *string { x := "666"; return &x }(),
		},
		{
			Name: "K3S_DATASTORE_ENDPOINT",
			Value: func() *string {
				x := "http://" + e.Framework.FrameworkName + "etcd" + e.Config.Domain + ":2379"
				return &x
			}(),
		},
		{
			Name:  "MESOS_SANDBOX_VAR",
			Value: &e.Config.MesosSandboxVar,
		},
	}

	// store mesos task in DB
	d, _ := json.Marshal(&cmd)
	logrus.Debug("Scheduled K3S Server: ", util.PrettyJSON(d))
	logrus.Info("Scheduled K3S Server")
	err := e.API.Redis.RedisClient.Set(e.API.Redis.RedisCTX, cmd.TaskName+":"+newTaskID, d, 0).Err()
	if err != nil {
		logrus.Error("Cloud not store Mesos Task in Redis: ", err)
	}
}

// CreateK3SServerString create the K3S_URL string
func (e *Scheduler)CreateK3SServerString() {
	server := "https://" + e.Framework.FrameworkName + "server" + e.Config.Domain + ":6443"

	e.Config.K3SServerURL = server
}

// IsK3SServerRunning check if the kubernetes server is already running
func (e *Scheduler)IsK3SServerRunning() bool {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://"+e.Config.M3SBootstrapServerHostname+":"+strconv.Itoa(e.Config.M3SBootstrapServerPort)+"/api/m3s/bootstrap/v0/status", nil)
	req.Close = true
	res, err := client.Do(req)

	if err != nil {
		logrus.Error("IsK3SServerRunning: Error 1: ", err, res)
		return false
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		logrus.Error("IsK3SServerRunning: Error Status is not 200")
		return false
	}

	content, err := ioutil.ReadAll(res.Body)

	if err != nil {
		logrus.Error("IsK3SServerRunning: Error 2: ", err, res)
		return false
	}

	if string(content) == "ok" {
		logrus.Debug("IsK3SServerRunning: True")
		e.Config.M3SStatus.API = "ok"
		return true
	}

	e.Config.M3SStatus.API = "nok"
	return false
}
