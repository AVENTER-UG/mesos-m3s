package api

import (
	"testing"
	cfg "github.com/AVENTER-UG/mesos-m3s/types"
)

func TestNewAPI(t *testing.T) {
	c := &cfg.Config{}
	f := &cfg.FrameworkConfig{}
	api := New(c, f)
	if api.Config != c {
		t.Error("Config not set correctly in API")
	}
	if api.Framework != f {
		t.Error("Framework not set correctly in API")
	}
}
