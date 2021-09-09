package api

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// V0GetKubeconfig will return the kubernetes config file
// example:
// curl -X GET 127.0.0.1:10000/v0/server/config'
func V0GetKubeconfig(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("HTTP GET V0GetKubeconfig")

	auth := CheckAuth(r, w)

	if !auth {
		return
	}

	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://"+config.M3SBootstrapServerHostname+":"+strconv.Itoa(config.M3SBootstrapServerPort)+"/api/k3s/v0/config", nil)
	req.Close = true
	res, err := client.Do(req)

	if err != nil {
		logrus.Error("GetKubeConfig: Error 1: ", err, res)
		return
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		logrus.Error("GetKubeConfig: Error Status is not 200")
		return
	}

	content, err := ioutil.ReadAll(res.Body)

	if err != nil {
		logrus.Error("GetKubeConfig: Error 2: ", err, res)
		return
	}

	// replace the localhost server string with the mesos agent hostname and dynamic port
	destURL := config.M3SBootstrapServerHostname + ":" + strconv.Itoa(config.K3SServerPort)
	kubconf := strings.Replace(string(content), "127.0.0.1:6443", destURL, -1)

	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Api-Service", "v0")
	w.Write([]byte(kubconf))

}
