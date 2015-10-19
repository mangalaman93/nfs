#!/bin/bash

# constants
OVS_BRIDGE=br-int

# get openstack environment variables
source ~/devstack/openrc admin

# $1 -> id
get_mac() {
  OUT_MAC=$(nova interface-list $1 | grep ACTIVE | awk '{print $10}')
}

# $1 -> id
get_port() {
  ID=$(nova show $1 | grep "| id" | awk '{print $4}')
  IF=$(docker exec $1-$ID ifconfig | grep Ethernet | awk '{print $1}')
  QVO_IF=qvo${IF:2:${#IF}}
  OUT_PORT=$(sudo ovs-ofctl show ${OVS_BRIDGE} | grep ${QVO_IF} | awk -F'[( )]' '{print $2}')
  if [[ -z ${OUT_PORT} ]]; then
    OUT_PORT=$(sudo ovs-ofctl show ${OVS_BRIDGE} | grep patch-tun | awk -F'[( )]' '{print $2}')
    if [[ -z ${OUT_PORT} ]]; then
      echo "error finding out port on the switch!"
      exit 1
    fi
  fi
}

if [ "$#" -eq 1 ]; then
  if [[ "$1" == "cleanup" ]]; then
    OVS_BRIDGE=br-int
    sudo ovs-ofctl dump-flows ${OVS_BRIDGE} | grep "priority=100" | while read -r LINE ; do
      sudo ovs-ofctl del-flows ${OVS_BRIDGE} $(echo ${LINE} | awk -F'priority=100,' '{print $2}' | awk -F',' '{print $1}') &> /dev/null
    done
    echo "${OVS_BRIDGE} rules cleaned up!"
    exit 0
  else
    echo "error!"
    echo "Usage: $0 cleanup"
    exit 1
  fi
fi

if [ "$#" -lt 3 ]; then
  echo "error!"
  echo "Usage: $0 <client> <router> <server>" >&2
  exit 1
fi

# client
get_mac $1
CLIENT_MAC=${OUT_MAC}
get_port $1
CLIENT_PORT=${OUT_PORT}
# router
get_mac $2
ROUTER_MAC=${OUT_MAC}
get_port $2
ROUTER_PORT=${OUT_PORT}
# server
get_mac $3
SERVER_MAC=${OUT_MAC}
# commands
sudo ovs-ofctl del-flows ${OVS_BRIDGE} dl_src=${CLIENT_MAC},dl_dst=${SERVER_MAC} &>/dev/null
sudo ovs-ofctl add-flow ${OVS_BRIDGE} priority=100,dl_src=${CLIENT_MAC}/ff:ff:ff:ff:ff:ff,dl_dst=${SERVER_MAC}/ff:ff:ff:ff:ff:ff,actions=mod_dl_dst=${ROUTER_MAC},output:${ROUTER_PORT} &>/dev/null
sudo ovs-ofctl add-flow ${OVS_BRIDGE} priority=100,dl_src=${SERVER_MAC}/ff:ff:ff:ff:ff:ff,dl_dst=${CLIENT_MAC}/ff:ff:ff:ff:ff:ff,actions=output:${CLIENT_PORT} &>/dev/null
exit
