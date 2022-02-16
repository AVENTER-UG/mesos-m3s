package mesos

import (
	"encoding/json"

	mesosutil "github.com/AVENTER-UG/mesos-util"
	mesosproto "github.com/AVENTER-UG/mesos-util/proto"
	"github.com/AVENTER-UG/util"
	"github.com/sirupsen/logrus"
)

func defaultResources(cmd mesosutil.Command) []mesosproto.Resource {
	CPU := "cpus"
	MEM := "mem"
	PORT := "ports"
	cpu := cmd.CPU
	mem := cmd.Memory

	res := []mesosproto.Resource{
		{
			Name:   CPU,
			Type:   mesosproto.SCALAR.Enum(),
			Scalar: &mesosproto.Value_Scalar{Value: cpu},
		},
		{
			Name:   MEM,
			Type:   mesosproto.SCALAR.Enum(),
			Scalar: &mesosproto.Value_Scalar{Value: mem},
		},
	}

	var portBegin, portEnd uint64

	if cmd.DockerPortMappings != nil {
		portBegin = uint64(cmd.DockerPortMappings[0].HostPort)
		portEnd = portBegin + uint64(len(cmd.DockerPortMappings)) - 1

		res = []mesosproto.Resource{
			{
				Name:   CPU,
				Type:   mesosproto.SCALAR.Enum(),
				Scalar: &mesosproto.Value_Scalar{Value: cpu},
			},
			{
				Name:   MEM,
				Type:   mesosproto.SCALAR.Enum(),
				Scalar: &mesosproto.Value_Scalar{Value: mem},
			},
			{
				Name: PORT,
				Type: mesosproto.RANGES.Enum(),
				Ranges: &mesosproto.Value_Ranges{
					Range: []mesosproto.Value_Range{
						{
							Begin: portBegin,
							End:   portEnd,
						},
					},
				},
			},
		}
	}

	return res
}

func prepareTaskInfoExecuteContainer(agent mesosproto.AgentID, cmd mesosutil.Command) []mesosproto.TaskInfo {
	d, _ := json.Marshal(&cmd)
	logrus.Debug("HandleOffers cmd: ", util.PrettyJSON(d))

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

	msg.Name = cmd.TaskName
	msg.TaskID = mesosproto.TaskID{
		Value: cmd.TaskID,
	}
	msg.AgentID = agent
	msg.Resources = defaultResources(cmd)

	if cmd.Command == "" {
		msg.Command = &mesosproto.CommandInfo{
			Shell:       &cmd.Shell,
			URIs:        cmd.Uris,
			Environment: &cmd.Environment,
		}
	} else {
		msg.Command = &mesosproto.CommandInfo{
			Shell:       &cmd.Shell,
			Value:       &cmd.Command,
			URIs:        cmd.Uris,
			Environment: &cmd.Environment,
		}
	}

	msg.Container = &mesosproto.ContainerInfo{
		Type:     contype,
		Volumes:  cmd.Volumes,
		Hostname: &cmd.Hostname,
		Docker: &mesosproto.ContainerInfo_DockerInfo{
			Image:        cmd.ContainerImage,
			Network:      networkMode,
			PortMappings: cmd.DockerPortMappings,
			Privileged:   &cmd.Privileged,
			Parameters:   cmd.DockerParameter,
		},
		NetworkInfos: cmd.NetworkInfo,
	}

	if cmd.Discovery != (mesosproto.DiscoveryInfo{}) {
		msg.Discovery = &cmd.Discovery
	}

	if cmd.Labels != nil {
		msg.Labels = &mesosproto.Labels{
			Labels: cmd.Labels,
		}
	}

	d, _ = json.Marshal(&msg)
	logrus.Debug("HandleOffers msg: ", util.PrettyJSON(d))

	return []mesosproto.TaskInfo{msg}
}
