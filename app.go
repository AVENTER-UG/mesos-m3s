package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/AVENTER-UG/mesos-m3s/api"
	"github.com/AVENTER-UG/mesos-m3s/controller"
	logrus "github.com/AVENTER-UG/mesos-m3s/logger"
	"github.com/AVENTER-UG/mesos-m3s/redis"
	"github.com/AVENTER-UG/mesos-m3s/scheduler"
	cfg "github.com/AVENTER-UG/mesos-m3s/types"

	util "github.com/AVENTER-UG/util/util"
)

// BuildVersion of m3s
var BuildVersion string

// GitVersion is the revision and commit number
var GitVersion string

// convert Base64 Encodes PEM Certificate to tls object
func decodeBase64Cert(pemCert string) []byte {
	sslPem, err := base64.URLEncoding.DecodeString(pemCert)
	if err != nil {
		logrus.Fatal("Error decoding SSL PEM from Base64: ", err.Error())
	}
	return sslPem
}

func main() {
	// Prints out current version
	var version bool
	flag.BoolVar(&version, "v", false, "Prints current version")
	flag.Parse()
	if version {
		fmt.Print(GitVersion)
		return
	}

	util.SetLogging(config.LogLevel, config.EnableSyslog, config.AppName)
	logrus.Println(config.AppName + " build " + BuildVersion + " git " + GitVersion)

	listen := fmt.Sprintf(":%s", framework.FrameworkPort)

	failoverTimeout := 5000.0
	checkpoint := true
	webuiurl := fmt.Sprintf("http://%s%s", framework.FrameworkHostname, listen)
	if config.SSLCrt != "" && config.SSLKey != "" {
		webuiurl = fmt.Sprintf("https://%s%s", framework.FrameworkHostname, listen)
	}

	framework.CommandChan = make(chan cfg.Command, 100)
	config.Hostname = framework.FrameworkHostname
	config.Listen = listen

	framework.State = map[string]cfg.State{}

	framework.FrameworkInfo.User = framework.FrameworkUser
	framework.FrameworkInfo.Name = framework.FrameworkName
	framework.FrameworkInfo.WebUiURL = &webuiurl
	framework.FrameworkInfo.FailoverTimeout = &failoverTimeout
	framework.FrameworkInfo.Checkpoint = &checkpoint
	framework.FrameworkInfo.Principal = &config.Principal
	framework.FrameworkInfo.Role = &framework.FrameworkRole

	//	config.FrameworkInfo.Capabilities = []mesosproto.FrameworkInfo_Capability{
	//		{Type: mesosproto.FrameworkInfo_Capability_RESERVATION_REFINEMENT},
	//	}

	// connect to redis
	r := redis.New(&config, &framework)
	if !r.Connect() {
		logrus.WithField("func", "main").Fatal("Could not connect to redis DB")
	}

	// load framework state from DB
	var oldFramework cfg.FrameworkConfig
	key := r.GetRedisKey(framework.FrameworkName + ":framework")
	if key != "" {
		json.Unmarshal([]byte(key), &oldFramework)

		framework.FrameworkInfo.ID = oldFramework.FrameworkInfo.ID
		framework.MesosStreamID = oldFramework.MesosStreamID
	}

	// restore variable data from the old config
	var oldconfig cfg.Config
	key = r.GetRedisKey(framework.FrameworkName + ":framework_config")
	if key != "" {
		json.Unmarshal([]byte(key), &oldconfig)
		config.K3SServerContainerPort = oldconfig.K3SServerContainerPort
		config.K3SServerHostname = oldconfig.K3SServerHostname
		config.K3SServerPort = oldconfig.K3SServerPort
		config.K3SServerURL = oldconfig.K3SServerURL
		config.K3SAgentMax = oldconfig.K3SAgentMax
		config.DSMax = oldconfig.DSMax

		// Add Missing Restore Values to handle the usecase of starting a hibernated cluster after reboot/framework restarts
		config.K3SServerMax = oldconfig.K3SServerMax
		config.DSMaxRestore = oldconfig.DSMaxRestore
		config.K3SServerMaxRestore = oldconfig.K3SServerMaxRestore
		config.K3SAgentMaxRestore = oldconfig.K3SAgentMaxRestore
	}

	r.SaveConfig(config)
	r.SaveFrameworkRedis(framework)

	// set current m3s version
	config.Version.M3SVersion.GitVersion = GitVersion
	config.Version.M3SVersion.BuildDate = BuildVersion

	// The Hostname should ever be set after reading the state file.
	framework.FrameworkInfo.Hostname = &framework.FrameworkHostname

	// get API
	a := api.New(&config, &framework)
	a.Redis = r

	server := &http.Server{
		Addr:              listen,
		Handler:           a.Commands(),
		ReadTimeout:       1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		TLSConfig: &tls.Config{
			ClientAuth: tls.RequestClientCert,
			MinVersion: tls.VersionTLS12,
		},
	}

	if config.SSLCrt != "" && config.SSLKey != "" {
		logrus.WithField("func", "main").Debug("Enable TLS")
		crt := decodeBase64Cert(config.SSLCrt)
		key := decodeBase64Cert(config.SSLKey)
		certs, err := tls.X509KeyPair(crt, key)
		if err != nil {
			logrus.WithField("func", "main.main").Fatal("TLS Server Error: ", err.Error())
		}
		server.TLSConfig.Certificates = []tls.Certificate{certs}
	}

	go func() {
		if config.SSLCrt != "" && config.SSLKey != "" {
			server.ListenAndServeTLS("", "")
		} else {
			server.ListenAndServe()
		}
	}()

	go loadPlugins(r)

	// create a kubernetes client
	k8 := controller.New(&config, &framework)
	k8.Redis = r

	// set kubernetes client to API
	a.Kubernetes = k8

	//	this loop is for resubscribtion purpose
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	// nolint:gosimple
	for {
		select {
		case <-ticker.C:
			e := scheduler.Subscribe(&config, &framework)
			e.API = a
			e.Redis = r
			e.Kubernetes = k8
			k8.CreateClient()
			go k8.CreateController()
			go k8.UnscheduleBeat()
			e.EventLoop()
		}
	}
}
