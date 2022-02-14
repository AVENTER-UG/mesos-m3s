#!/bin/bash

cat /etc/resolv.conf

apt-get update -y
apt-get install -y jq containerd dnsmasq containernetworking-plugins tcpdump curl inetutils-ping iptables fuse-overlayfs procps bash iproute2 dnsutils net-tools systemctl
mkdir -p /etc/cni/net.d

export KUBERNETES_VERSION=v1.21.1
export INSTALL_K3S_VERSION=+k3s1
export INSTALL_K3S_SKIP_ENABLE=true
export INSTALL_K3S_SKIP_START=true
export KUBECONFIG=/kubeconfig.yaml

## Export json as environment variables
## example: MESOS_SANDBOX_VAR='{ "CUSTOMER":"test-ltd" }'
## echo  >> test-ltd
for s in $(echo  | jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" ); do
  export
done


update-alternatives --set iptables /usr/sbin/iptables-legacy
curl -sfL https://get.k3s.io | sh -
curl https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/dev/bootstrap/dashboard_auth.yaml > /dashboard_auth.yaml
curl https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/dev/bootstrap/dashboard_traefik.yaml > /dashboard_traefik.yaml
curl https://raw.githubusercontent.com/kubernetes/dashboard/v2.2.0/aio/deploy/recommended.yaml > /dashboard.yaml
if [[ "" == "server" ]]
then
  curl -L http://dl.k8s.io/release//bin/linux/amd64/kubectl > /kubectl
  curl https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/dev/bootstrap/server > /server
  curl https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/dev/bootstrap/update.sh > /update
  chmod +x /kubectl
  chmod +x /server
  chmod +x /update
  exec /server &
fi
if [[ "" == "agent" ]]
then
  echo "These place you can use to manipulate the configuration of containerd (as example)."
fi


echo $1
$1
