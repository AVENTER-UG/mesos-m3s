package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/AVENTER-UG/util"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	cfg "github.com/AVENTER-UG/mesos-m3s/types"
)

// MinVersion is the version number of this program
var MinVersion string

// DashboardInstalled is true if the dashboard is already installed
var DashboardInstalled bool

// TraefikDashboardInstalled is true if the traefik dashboard is installed
var TraefikDashboardInstalled bool

// Commands is the main function of this package
func Commands() *mux.Router {
	// Connect with database

	rtr := mux.NewRouter()
	rtr.HandleFunc("/versions", APIVersions).Methods("GET")
	rtr.HandleFunc("/update", APIUpdate).Methods("PUT")
	rtr.HandleFunc("/status", APIHealth).Methods("GET")
	rtr.HandleFunc("/api/k3s/v0/config", APIGetKubeConfig).Methods("GET")
	rtr.HandleFunc("/api/k3s/v0/version", APIGetKubeVersion).Methods("GET")
	rtr.HandleFunc("/status?verbose", APIStatus).Methods("GET")

	return rtr
}

// APIVersions give out a list of Versions
func APIVersions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Api-Service", "-")
	w.Write([]byte("/api/k3s/v0"))
}

// APIUpdate do a update of the bootstrap server
func APIUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Api-Service", "v0")

	// check first if there is a update
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/master/.version.json", nil)
	req.Close = true
	res, err := client.Do(req)

	if err != nil {
		logrus.Error("APIUpdate: Error 1: ", err, res)
		return
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		w.WriteHeader(http.StatusInternalServerError)
		logrus.Error("APIUpdate: Error Status is not 200")
		return
	}

	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logrus.Error("APIUpdate: Error 2: ", err, res)
		return
	}

	var version cfg.Version
	err = json.Unmarshal(body, &version)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logrus.Error("APIUpdate: Error 3: ", err, body)
		return
	}

	// check if the current Version diffs to the online version. If yes, then start the update.
	if version.BootstrapBuild != MinVersion {
		w.Write([]byte("Start bootstrap server update"))
		logrus.Info("Start update")
		// #nosec: G204
		stdout, err := exec.Command("/mnt/mesos/sandbox/update", strconv.Itoa(os.Getpid())).Output()
		if err != nil {
			logrus.Error("Do update", err, stdout)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		w.Write([]byte("No update for the bootstrap server"))
	}
}

// APIGetKubeConfig get out the kubernetes config file
func APIGetKubeConfig(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadFile("/mnt/mesos/sandbox/kubeconfig.yaml")
	if err != nil {
		logrus.Error("Error reading file:", err)
		w.Write([]byte("Error reading kubeconfig.yaml"))
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Api-Service", "v0")

		w.Write(content)
	}
}

// APIGetKubeVersion get out the kubernetes version number
func APIGetKubeVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Api-Service", "v0")

	stdout, err := exec.Command("/mnt/mesos/sandbox/kubectl", "version", "-o=json").Output()
	if err != nil {
		logrus.Error("Get Kubernetes Version: ", err, stdout)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(stdout)
}

// APIHealth give out the status of the kubernetes server
func APIHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Api-Service", "v0")

	logrus.Debug("Health Check")

	// check if the kubernetes server is working
	stdout, err := exec.Command("/mnt/mesos/sandbox/kubectl", "get", "--raw=/livez/ping").Output()

	if err != nil {
		logrus.Error("Health to Kubernetes Server: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if string(stdout) == "ok" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))

		// if kubernetes server is running and the dashboard is not installed, then do it
		if !DashboardInstalled {
			deployDashboard()
		}
		// if kubernetes server is running and the traefik dashboard is not installed, then do it
		if !TraefikDashboardInstalled {
			deployTraefikDashboard()
		}
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// deployDashboard will deploy the kubernetes dashboard
// if the server is in the running state
func deployDashboard() {
	err := exec.Command("/mnt/mesos/sandbox/kubectl", "apply", "-f", "/mnt/mesos/sandbox/dashboard.yaml").Run()
	logrus.Info("Install Kubernetes Dashboard")

	if err != nil {
		logrus.Error("Install Kubernetes Dashboard: ", err)
		return
	}

	err = exec.Command("/mnt/mesos/sandbox/kubectl", "apply", "-f", "/mnt/mesos/sandbox/dashboard_auth.yaml").Run()

	if err != nil {
		logrus.Error("Install Kubernetes Dashboard Auth: ", err)
		return
	}

	logrus.Info("Install Kubernetes Dashboard: Done")
	DashboardInstalled = true
}

// deployTraefikDashboard will deploy the traefik dashboard
// if the server is in the running state
func deployTraefikDashboard() {
	err := exec.Command("/mnt/mesos/sandbox/kubectl", "apply", "-f", "/mnt/mesos/sandbox/dashboard_traefik.yaml").Run()
	logrus.Info("Install Traefik Dashboard")

	if err != nil {
		logrus.Error("Install Traefik Dashboard: ", err)
		return
	}

	logrus.Info("Install Traefik Dashboard: Done")
	TraefikDashboardInstalled = true
}

// APIStatus give out the status of the kubernetes server
func APIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Api-Service", "v0")

	logrus.Debug("Status Information")

	// check if the kubernetes server is working
	stdout, err := exec.Command("/mnt/mesos/sandbox/kubectl", "get", "--raw='/readyz?verbose'").Output()

	if err != nil {
		logrus.Error("Health to Kubernetes Server: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(stdout)
}

func main() {
	util.SetLogging("INFO", false, "GO-K3S-API")

	bind := flag.String("bind", "0.0.0.0", "The IP address to bind")
	port := flag.String("port", "10422", "The port to listen")

	logrus.Println("GO-K3S-API build "+MinVersion, *bind, *port)

	DashboardInstalled = false

	http.Handle("/", Commands())

	if err := http.ListenAndServe(*bind+":"+*port, nil); err != nil {
		logrus.Fatalln("ListenAndServe: ", err)
	}
}
