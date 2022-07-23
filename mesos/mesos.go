package mesos

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strings"

	api "github.com/AVENTER-UG/mesos-m3s/api"
	cfg "github.com/AVENTER-UG/mesos-m3s/types"
	mesosutil "github.com/AVENTER-UG/mesos-util"
	mesosproto "github.com/AVENTER-UG/mesos-util/proto"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/sirupsen/logrus"
)

// Scheduler include all the current vars and global config
type Scheduler struct {
	Config    *cfg.Config
	Framework *mesosutil.FrameworkConfig
	Client    *http.Client
	Req       *http.Request
	API       *api.API
}

// Marshaler to serialize Protobuf Message to JSON
var marshaller = jsonpb.Marshaler{
	EnumsAsInts: false,
	Indent:      " ",
	OrigName:    true,
}

// Subscribe to the mesos backend
func Subscribe(cfg *cfg.Config, frm *mesosutil.FrameworkConfig) *Scheduler {
	e := &Scheduler{
		Config:    cfg,
		Framework: frm,
	}

	subscribeCall := &mesosproto.Call{
		FrameworkID: e.Framework.FrameworkInfo.ID,
		Type:        mesosproto.Call_SUBSCRIBE,
		Subscribe: &mesosproto.Call_Subscribe{
			FrameworkInfo: &e.Framework.FrameworkInfo,
		},
	}
	logrus.Debug(subscribeCall)
	body, _ := marshaller.MarshalToString(subscribeCall)
	logrus.Debug(body)
	client := &http.Client{}
	// #nosec G402
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: e.Config.SkipSSL},
	}

	protocol := "https"
	if !e.Framework.MesosSSL {
		protocol = "http"
	}
	req, _ := http.NewRequest("POST", protocol+"://"+e.Framework.MesosMasterServer+"/api/v1/scheduler", bytes.NewBuffer([]byte(body)))
	req.Close = true
	req.SetBasicAuth(e.Framework.Username, e.Framework.Password)
	req.Header.Set("Content-Type", "application/json")

	e.Req = req
	e.Client = client

	return e
}

// EventLoop is the main loop for the mesos events.
func (e *Scheduler) EventLoop() {
	res, err := e.Client.Do(e.Req)

	if err != nil {
		logrus.Fatal(err)
	}
	defer res.Body.Close()

	reader := bufio.NewReader(res.Body)

	line, _ := reader.ReadString('\n')
	_ = strings.TrimSuffix(line, "\n")

	// initialstart
	if e.Framework.MesosStreamID == "" {
		e.StartEtcd("")
	}

	go e.HeartbeatLoop()

	for {
		// Read line from Mesos
		line, _ = reader.ReadString('\n')
		line = strings.TrimSuffix(line, "\n")
		// Read important data
		var event mesosproto.Event // Event as ProtoBuf
		err := jsonpb.UnmarshalString(line, &event)
		if err != nil {
			logrus.Error(err)
		}
		logrus.Debug("Subscribe Got: ", event.GetType())

		switch event.Type {
		case mesosproto.Event_SUBSCRIBED:
			logrus.Debug(event)
			logrus.Info("Subscribed")
			logrus.Info("FrameworkId: ", event.Subscribed.GetFrameworkID())
			e.Framework.FrameworkInfo.ID = event.Subscribed.GetFrameworkID()
			e.Framework.MesosStreamID = res.Header.Get("Mesos-Stream-Id")
			d, _ := json.Marshal(&e.Framework)
			err = e.API.Redis.RedisClient.Set(e.API.Redis.RedisCTX, e.Framework.FrameworkName+":framework", d, 0).Err()
			if err != nil {
				logrus.Error("Framework save config and state into redis Error: ", err)
			}
			e.API.SaveConfig()
		case mesosproto.Event_UPDATE:
			e.HandleUpdate(&event)
			// save configuration
			e.API.SaveConfig()
		case mesosproto.Event_OFFERS:
			// Search Failed containers and restart them
			logrus.Debug("Offer Got")
			err = e.HandleOffers(event.Offers)
			if err != nil {
				logrus.Error("Switch Event HandleOffers: ", err)
			}
		}
	}
}

// Generate random host portnumber
func (e *Scheduler)getRandomHostPort(num int) uint32 {
	// search two free ports
	for i := e.Framework.PortRangeFrom; i < e.Framework.PortRangeTo; i++ {
		port := uint32(i)
		use := false
		for x := 0; x < num; x++ {
			if e.portInUse(port+uint32(x), "server") || e.portInUse(port+uint32(x), "agent") {
				tmp := use || true
				use = tmp
				x = num
			}

			tmp := use || false
			use = tmp
		}
		if !use {
			return port
		}
	}
	return 0
}

// Check if the port is already in use
func (e *Scheduler)portInUse(port uint32, service string) bool {
	// get all running services
	logrus.Debug("Check if port is in use: ", port, service)
	keys := e.API.GetAllRedisKeys(e.Framework.FrameworkName + ":" + service + ":*")
	for keys.Next(e.API.Redis.RedisCTX) {
		// get the details of the current running service
		key := e.API.GetRedisKey(keys.Val())
		var task mesosutil.Command
		json.Unmarshal([]byte(key), &task)

		// check if the given port is already in use
		ports := task.Discovery.GetPorts()
		if ports != nil {
			for _, hostport := range ports.GetPorts() {
				if hostport.Number == port {
					logrus.Debug("Port in use: ", port, service)
					return true
				}
			}
		}
	}
	return false
}
