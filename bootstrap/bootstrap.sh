#!/bin/bash

cat /etc/resolv.conf

apt-get update -y
mkdir -p /etc/cni/net.d

export KUBERNETES_VERSION=v1.21.1
export INSTALL_K3S_VERSION=$KUBERNETES_VERSION+k3s1
export INSTALL_K3S_SKIP_ENABLE=true
export INSTALL_K3S_SKIP_START=true
export KUBECONFIG=$MESOS_SANDBOX/kubeconfig.yaml
export BRANCH=dev

## Export json as environment variables
## example: MESOS_SANDBOX_VAR='{ "CUSTOMER":"test-ltd" }'
## echo $CUSTOMER >> test-ltd
for s in $(echo $MESOS_SANDBOX_VAR | jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" ); do
  export $s
done


set -o errexit -o nounset -o pipefail -o verbose

CGROUP=$(grep memory /proc/1/cgroup | cut -d: -f3)

cat <<EOF > "/etc/docker/env"
CGROUP_PARENT=${CGROUP}/docker
EOF

## dockerd is a part of the uses avhost/ubuntu-m3s:focal docker image
exec /usr/local/bin/dockerd &

curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=${INSTALL_K3S_VERSION} INSTALL_K3S_SKIP_ENABLE=${INSTALL_K3S_SKIP_ENABLE=$} INSTALL_K3S_SKIP_START=${INSTALL_K3S_SKIP_START} sh -s - --docker
curl https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/${BRANCH}/bootstrap/dashboard_auth.yaml > $MESOS_SANDBOX/dashboard_auth.yaml
curl https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/${BRANCH}/bootstrap/dashboard_traefik.yaml > $MESOS_SANDBOX/dashboard_traefik.yaml
curl https://raw.githubusercontent.com/kubernetes/dashboard/v2.2.0/aio/deploy/recommended.yaml > $MESOS_SANDBOX/dashboard.yaml
if [[ "$K3SFRAMEWORK_TYPE" == "server" ]]
then
  curl -L http://dl.k8s.io/release/$KUBERNETES_VERSION/bin/linux/amd64/kubectl > $MESOS_SANDBOX/kubectl
  curl https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/${BRANCH}/bootstrap/server > $MESOS_SANDBOX/server
  curl https://raw.githubusercontent.com/AVENTER-UG/mesos-m3s/${BRANCH}/bootstrap/update.sh > $MESOS_SANDBOX/update
  chmod +x $MESOS_SANDBOX/kubectl
  chmod +x $MESOS_SANDBOX/server
  chmod +x $MESOS_SANDBOX/update
  exec $MESOS_SANDBOX/server &
fi
if [[ "$K3SFRAMEWORK_TYPE" == "agent" ]]
then
  echo "These place you can use to manipulate the configuration of containerd (as example)."
fi


echo $1
$1
