package scheduler

import (
	"encoding/json"

	logrus "github.com/AVENTER-UG/mesos-m3s/logger"
	mesosproto "github.com/AVENTER-UG/mesos-m3s/proto"
	cfg "github.com/AVENTER-UG/mesos-m3s/types"
	"github.com/AVENTER-UG/util/util"
)

// default resources of the mesos task
func (e *Scheduler) defaultResources(cmd *cfg.Command) []*mesosproto.Resource {
	CPU := "cpus"
	MEM := "mem"
	PORT := "ports"
	DISK := "disk"
	cpu := cmd.CPU
	mem := cmd.Memory
	disk := cmd.Disk

	res := []*mesosproto.Resource{
		{
			Name:   &CPU,
			Type:   mesosproto.Value_SCALAR.Enum(),
			Scalar: &mesosproto.Value_Scalar{Value: &cpu},
		},
		{
			Name:   &MEM,
			Type:   mesosproto.Value_SCALAR.Enum(),
			Scalar: &mesosproto.Value_Scalar{Value: &mem},
		},
		{
			Name:   &DISK,
			Type:   mesosproto.Value_SCALAR.Enum(),
			Scalar: &mesosproto.Value_Scalar{Value: &disk},
		},
	}

	if cmd.DockerPortMappings != nil {
		for _, p := range cmd.DockerPortMappings {
			port := mesosproto.Resource{
				Name: &PORT,
				Type: mesosproto.Value_RANGES.Enum(),
				Ranges: &mesosproto.Value_Ranges{
					Range: []*mesosproto.Value_Range{
						{
							Begin: util.Uint64ToPointer(uint64(*p.HostPort)),
							End:   util.Uint64ToPointer(uint64(*p.HostPort)),
						},
					},
				},
			}
			res = append(res, &port)
		}
	}

	return res
}

// default values of the mesos tasks
func (e *Scheduler) defaultCommand(taskID string) cfg.Command {
	var cmd cfg.Command

	cmd.TaskID = e.getTaskID(taskID)

	cni := e.Framework.MesosCNI
	if e.Framework.MesosCNI == "" {
		if e.Config.DockerCNI != "bridge" {
			cmd.NetworkMode = "user"
			cni = e.Config.DockerCNI
		}
	}
	cmd.NetworkInfo = []*mesosproto.NetworkInfo{{
		Name: &cni,
	}}

	cmd.ContainerType = "DOCKER"

	return cmd
}

func (e *Scheduler) prepareTaskInfoExecuteContainer(agent *mesosproto.AgentID, cmd *cfg.Command) []*mesosproto.TaskInfo {
	contype := mesosproto.ContainerInfo_DOCKER.Enum()

	// Set Container Network Mode
	networkMode := mesosproto.ContainerInfo_DockerInfo_BRIDGE.Enum()

	if cmd.NetworkMode == "host" {
		networkMode = mesosproto.ContainerInfo_DockerInfo_HOST.Enum()
	}
	if cmd.NetworkMode == "none" {
		networkMode = mesosproto.ContainerInfo_DockerInfo_NONE.Enum()
	}
	if cmd.NetworkMode == "user" {
		networkMode = mesosproto.ContainerInfo_DockerInfo_USER.Enum()
	}
	if cmd.NetworkMode == "bridge" {
		networkMode = mesosproto.ContainerInfo_DockerInfo_BRIDGE.Enum()
	}

	var msg mesosproto.TaskInfo

	msg.Name = &cmd.TaskName
	msg.TaskId = &mesosproto.TaskID{
		Value: &cmd.TaskID,
	}
	msg.AgentId = agent
	msg.Resources = e.defaultResources(cmd)

	if cmd.Command == "" {
		msg.Command = &mesosproto.CommandInfo{
			Shell:       &cmd.Shell,
			Arguments:   cmd.Arguments,
			Uris:        cmd.Uris,
			Environment: cmd.Environment,
		}
	} else {
		msg.Command = &mesosproto.CommandInfo{
			Shell:       &cmd.Shell,
			Value:       &cmd.Command,
			Arguments:   cmd.Arguments,
			Uris:        cmd.Uris,
			Environment: cmd.Environment,
		}
	}

	msg.Container = &mesosproto.ContainerInfo{
		Type:     contype,
		Volumes:  cmd.Volumes,
		Hostname: &cmd.Hostname,
		Docker: &mesosproto.ContainerInfo_DockerInfo{
			Image:          util.StringToPointer(cmd.ContainerImage),
			Network:        networkMode,
			PortMappings:   cmd.DockerPortMappings,
			Privileged:     &cmd.Privileged,
			Parameters:     cmd.DockerParameter,
			ForcePullImage: func() *bool { x := true; return &x }(),
		},
		NetworkInfos: cmd.NetworkInfo,
	}

	if cmd.Discovery != (&mesosproto.DiscoveryInfo{}) {
		msg.Discovery = cmd.Discovery
	}

	if cmd.Labels != nil {
		msg.Labels = &mesosproto.Labels{
			Labels: cmd.Labels,
		}
	}

	if cmd.EnableHealthCheck {
		msg.HealthCheck = cmd.Health
	}

	if e.Config.EnforceMesosTaskLimits {
		msg.Limits = map[string]*mesosproto.Value_Scalar{
			"cpus": {Value: &cmd.CPULimit},
			"mem":  {Value: &cmd.MemoryLimit},
		}
	}

	d, _ := json.Marshal(&msg)
	logrus.WithField("func", "scheduler.prepareTaskInfoExecuteContainer").Debug("HandleOffers msg: ", util.PrettyJSON(d))

	return []*mesosproto.TaskInfo{&msg}
}
