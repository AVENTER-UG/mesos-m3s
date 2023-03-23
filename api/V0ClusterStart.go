package api

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

// V0ClusterStart - Start the cluster after shutdown
// example:
// curl -X PUT 127.0.0.1:10000/v0/cluster/start
func (e *API) V0ClusterStart(w http.ResponseWriter, r *http.Request) {
	logrus.WithField("func", "api.V0ClusterStart").Debug("Start Cluster")

	if !e.CheckAuth(r, w) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if e.Config.DSMax != 0 && e.Config.K3SServerMax != 0 && e.Config.K3SAgentMax != 0 {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	e.clusterStart()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Api-Service", "v0")
}

func (e *API) clusterStart() {
	logrus.WithField("func", "api.clusterStart").Debug("Start Cluster")
	e.scaleDatastore(e.DSMaxRestore)
	e.scaleServer(e.K3SServerMaxRestore)
	e.scaleAgent(e.K3SAgentMaxRestore)
}
