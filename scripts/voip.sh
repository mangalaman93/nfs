#!/bin/bash

if [ "$#" -lt 1 ]; then
  echo "error!"
  echo "Usage: $0 start|stop" >&2
  exit 1
fi

# get openstack environment variables
source ~/devstack/openrc admin

# influxdb config
INFLUXDB_IP=
INFLUXDB_PORT=
INFLUXDB_USER=
INFLUXDB_PASS=
CUR_HOST=jedi054
OTH_HOST=jedi055

# $1 -> id
find_ip() {
  OUT_IP=$(nova show $1 | grep network | tr -d [:space:] | awk -F'[,|]' '{print $3}')
  if [[ $OUT_IP == *":"* ]]; then
    OUT_IP=$(nova show $1 | grep network | tr -d [:space:] | awk -F'[,|]' '{print $4}')
  fi
}

stop_exp() {
  HOST=$(hostname)

  echo "stop cadvisor"
  docker kill $HOST-cadvisor &>/dev/null
  docker rm $HOST-cadvisor &>/dev/null

  echo "stop monsipp"
  sudo kill $(pidof monsipp) &>/dev/null

  echo "delete containers"
  nova delete sipp-server sipp-client snort &>/dev/null

  echo "delete volumes"
  sudo rm -rf /var/lib/docker/volumes/*

  echo "cleanup done!"
}

# $1 -> id, $2 -> host
check_host() {
  THIS_HOST=$(nova show $1 | grep "OS-EXT-SRV-ATTR:hypervisor_hostname" | awk '{print $4}')
  if [[ $THIS_HOST != $2 ]]; then
    echo "Error: $1 started on unexpected host $THIS_HOST instead of $2!"
    stop_exp
    exit 1
  fi
}

start_exp_on_cur() {
  docker inspect $CUR_HOST-cadvisor &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: cadvisor is already running!"
    exit 1
  fi

  pidof monsipp &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: monsipp is already running!"
    exit 1
  fi

  nova show sipp-server &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: sipp-server is already running!"
    exit 1
  fi

  nova show sipp-client &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: sipp-client is already running!"
    exit 1
  fi

  nova show snort &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: snort is already running!"
    exit 1
  fi

  # run cadvisor
  docker run -d --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro --name=$CUR_HOST-cadvisor mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_user=$INFLUXDB_USER -storage_driver_password=$INFLUXDB_PASS -storage_driver_host=$INFLUXDB_IP:$INFLUXDB_PORT > /dev/null

  # run monsipp
  cd ~/nfs/ && sudo ./monsipp -d $INFLUXDB_IP:$INFLUXDB_PORT:$INFLUXDB_USER:$INFLUXDB_PASS && cd

  # run sipp server
  echo "running sipp-server"
  nova boot --image mangalaman93/sipp --meta ARGS="-sn uas" --availability-zone regionOne:$CUR_HOST --flavor c1.medium sipp-server > /dev/null
  sleep 3
  check_host sipp-server $CUR_HOST
  find_ip sipp-server
  SERVER_IP=$OUT_IP
  echo "server: $SERVER_IP"

  # run snort
  echo "running snort"
  nova boot --image mangalaman93/snort --meta OPT_CAP_ADD=NET_ADMIN --availability-zone regionOne:$OTH_HOST --flavor c1.tiny snort > /dev/null
  sleep 3
  check_host snort $OTH_HOST
  find_ip snort
  ROUTER_IP=$OUT_IP
  echo "router: $ROUTER_IP"

  # run sipp-client
  echo "running sipp-client"
  nova boot --image mangalaman93/sipp --meta ARGS="-sn uac $SERVER_IP:5060" --availability-zone regionOne:$CUR_HOST --flavor c1.medium sipp-client > /dev/null
  sleep 3
  check_host sipp-client $CUR_HOST
  CLIENT_ID=$(nova show sipp-client | grep "| id" | awk '{print $4}')
  echo "client-id: $CLIENT_ID"
  find_ip sipp-client
  CLIENT_IP=$OUT_IP
  echo "client: $CLIENT_IP"

  # settings
  echo "setting up routes"
  FULL_CLIENT_DOCKER_ID=$(docker inspect --format '{{ .Id }}' nova-$CLIENT_ID)
  echo "docker-client-id: $FULL_CLIENT_DOCKER_ID"
  sudo ip netns exec $FULL_CLIENT_DOCKER_ID ip route add $SERVER_IP via $ROUTER_IP

  echo "running checks..."
  docker inspect $CUR_HOST-cadvisor &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: cadvisor did not run!"
  fi

  pidof monsipp &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: monsipp did not run!"
  fi

  nova show sipp-server &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: sipp-server did not run!"
  fi

  nova show sipp-client &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: sipp-client did not run!"
  fi

  nova show snort &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: snort did not run!"
  fi
}

start_exp_on_oth() {
  docker inspect $OTH_HOST-cadvisor &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: cadvisor is already running!"
    exit 1
  fi

  pidof monsipp &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: monsipp is not running!"
    exit 1
  fi

  nova show sipp-server &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: sipp-server is not running!"
    exit 1
  fi

  nova show sipp-client &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: sipp-client is not running!"
    exit 1
  fi

  nova show snort &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: snort is not running!"
    exit 1
  fi

  SNORT_ID=$(nova show snort | grep "| id" | awk '{print $4}')
  echo "snort-id: $SNORT_ID"
  docker inspect nova-$SNORT_ID &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: snort is not running on $OTH_HOST"
    exit 1
  fi

  # run cadvisor
  docker run -d --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro --name=$OTH_HOST-cadvisor mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_user=$INFLUXDB_USER -storage_driver_password=$INFLUXDB_PASS -storage_driver_host=$INFLUXDB_IP:$INFLUXDB_PORT > /dev/null

  # run monsipp
  cd ~/nfs/ && sudo ./monsipp -d $INFLUXDB_IP:$INFLUXDB_PORT:$INFLUXDB_USER:$INFLUXDB_PASS && cd

  find_ip sipp-server
  SERVER_IP=$OUT_IP
  echo "server: $SERVER_IP"
  find_ip sipp-client
  CLIENT_IP=$OUT_IP
  echo "client: $CLIENT_IP"
  find_ip snort
  ROUTER_IP=$OUT_IP
  echo "router: $ROUTER_IP"

  FULL_SNORT_DOCKER_ID=$(docker inspect --format '{{ .Id }}' nova-$SNORT_ID)
  echo "snort-docker-id: $FULL_SNORT_DOCKER_ID"
  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -t nat -A PREROUTING -d $ROUTER_IP -s $CLIENT_IP -j DNAT --to-destination $SERVER_IP
  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -t nat -I POSTROUTING -s $CLIENT_IP -j SNAT --to-source $ROUTER_IP
  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -A FORWARD -d $SERVER_IP -i eth0 -j ACCEPT

  echo "running checks..."
  docker inspect $OTH_HOST-cadvisor &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: cadvisor did not run!"
  fi

  pidof monsipp &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: monsipp did not run!"
  fi
}

case $1 in
  "start")
    case $(hostname) in
      $CUR_HOST)
        start_exp_on_cur
        sudo iptables -F
        ;;
      $OTH_HOST)
        start_exp_on_oth
        sudo iptables -F
        sudo sysctl -w net.bridge.bridge-nf-call-iptables=0
        ;;
      *)
        echo "Error: Unknown host!"
        exit 1
        ;;
    esac
    ;;
  "stop")
    stop_exp
    ;;
  *)
    echo "Error: Unknown command!"
    exit 1
    ;;
esac
