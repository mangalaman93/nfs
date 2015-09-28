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
  nova delete sipp-server1 sipp-server2 sipp-client1 sipp-client2 snort &>/dev/null

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

# $1 -> id
err_if_running() {
  nova show $1 &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: $1 is already running!"
    exit 1
  fi
}

# $1 -> id
err_if_not_running() {
  nova show $1 &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: $1 did not run!"
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

  err_if_running sipp-server1
  err_if_running sipp-server2
  err_if_running sipp-client1
  err_if_running sipp-client2
  err_if_running snort

  # run monsipp
  cd ~/nfs/ && sudo ./monsipp -d $INFLUXDB_IP:$INFLUXDB_PORT:$INFLUXDB_USER:$INFLUXDB_PASS

  # wait for monsipp to start on other server
  echo -n "run the same script on $OTH_HOST and press enter"
  read text

  # run sipp server1
  echo "running sipp-server1"
  nova boot --image mangalaman93/sipp --meta ARGS="-sn uas" --availability-zone regionOne:$OTH_HOST --flavor c1.medium sipp-server1 > /dev/null
  sleep 3
  check_host sipp-server1 $OTH_HOST
  find_ip sipp-server1
  SERVER1_IP=$OUT_IP
  echo "server1: $SERVER1_IP"

  # run sipp server2
  echo "running sipp-server2"
  nova boot --image mangalaman93/sipp --meta ARGS="-sn uas" --availability-zone regionOne:$OTH_HOST --flavor c1.medium sipp-server2 > /dev/null
  sleep 3
  check_host sipp-server2 $OTH_HOST
  find_ip sipp-server2
  SERVER2_IP=$OUT_IP
  echo "server2: $SERVER2_IP"

  # run snort
  echo "running snort"
  nova boot --image mangalaman93/snort --meta OPT_CAP_ADD=NET_ADMIN --availability-zone regionOne:$OTH_HOST --flavor c1.tiny snort > /dev/null
  sleep 3
  check_host snort $OTH_HOST
  find_ip snort
  ROUTER_IP=$OUT_IP
  echo "router: $ROUTER_IP"

  # run sipp-client1
  echo "running sipp-client1"
  nova boot --image mangalaman93/sipp --meta ARGS="-sn uac $SERVER1_IP:5060" --availability-zone regionOne:$CUR_HOST --flavor c1.medium sipp-client1 > /dev/null
  sleep 3
  check_host sipp-client1 $CUR_HOST
  CLIENT1_ID=$(nova show sipp-client1 | grep "| id" | awk '{print $4}')
  echo "client1-id: $CLIENT1_ID"
  find_ip sipp-client1
  CLIENT1_IP=$OUT_IP
  echo "client1: $CLIENT1_IP"

  # run sipp-client2
  echo "running sipp-client2"
  nova boot --image mangalaman93/sipp --meta ARGS="-sn uac $SERVER2_IP:5060" --availability-zone regionOne:$CUR_HOST --flavor c1.medium sipp-client2 > /dev/null
  sleep 3
  check_host sipp-client2 $CUR_HOST
  CLIENT2_ID=$(nova show sipp-client2 | grep "| id" | awk '{print $4}')
  echo "client2-id: $CLIENT2_ID"
  find_ip sipp-client2
  CLIENT2_IP=$OUT_IP
  echo "client2: $CLIENT2_IP"

  # run cadvisor
  docker run -d --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro --name=$CUR_HOST-cadvisor mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_user=$INFLUXDB_USER -storage_driver_password=$INFLUXDB_PASS -storage_driver_host=$INFLUXDB_IP:$INFLUXDB_PORT > /dev/null

  # settings
  echo "setting up routes"
  FULL_CLIENT1_DOCKER_ID=$(docker inspect --format '{{ .Id }}' sipp-client1-$CLIENT1_ID)
  echo "docker-client1-id: $FULL_CLIENT1_DOCKER_ID"
  sudo ip netns exec $FULL_CLIENT1_DOCKER_ID ip route add $SERVER1_IP via $ROUTER_IP

  FULL_CLIENT2_DOCKER_ID=$(docker inspect --format '{{ .Id }}' sipp-client2-$CLIENT2_ID)
  echo "docker-client2-id: $FULL_CLIENT2_DOCKER_ID"
  sudo ip netns exec $FULL_CLIENT2_DOCKER_ID ip route add $SERVER2_IP via $ROUTER_IP

  echo "running checks..."
  docker inspect $CUR_HOST-cadvisor &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: cadvisor did not run!"
  fi

  pidof monsipp &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: monsipp did not run!"
  fi

  err_if_not_running sipp-server1
  err_if_not_running sipp-server2
  err_if_not_running sipp-client1
  err_if_not_running sipp-client2
  err_if_not_running snort
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

  # run monsipp
  cd ~/nfs/ && sudo ./monsipp -d $INFLUXDB_IP:$INFLUXDB_PORT:$INFLUXDB_USER:$INFLUXDB_PASS

  # wait for containers to start
  echo -n "press enter when script is done running on $CUR_HOST"
  read text

  err_if_not_running sipp-server1
  err_if_not_running sipp-server2
  err_if_not_running sipp-client1
  err_if_not_running sipp-client2
  err_if_not_running snort

  SNORT_ID=$(nova show snort | grep "| id" | awk '{print $4}')
  echo "snort-id: $SNORT_ID"
  docker inspect snort-$SNORT_ID &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: snort is not running on $OTH_HOST"
    exit 1
  fi

  # run cadvisor
  docker run -d --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro --name=$OTH_HOST-cadvisor mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_user=$INFLUXDB_USER -storage_driver_password=$INFLUXDB_PASS -storage_driver_host=$INFLUXDB_IP:$INFLUXDB_PORT > /dev/null

  find_ip sipp-server1
  SERVER1_IP=$OUT_IP
  echo "server1: $SERVER1_IP"
  find_ip sipp-client1
  CLIENT1_IP=$OUT_IP
  echo "client1: $CLIENT1_IP"
  find_ip sipp-server2
  SERVER2_IP=$OUT_IP
  echo "server2: $SERVER2_IP"
  find_ip sipp-client2
  CLIENT2_IP=$OUT_IP
  echo "client2: $CLIENT2_IP"
  find_ip snort
  ROUTER_IP=$OUT_IP
  echo "router: $ROUTER_IP"

  FULL_SNORT_DOCKER_ID=$(docker inspect --format '{{ .Id }}' snort-$SNORT_ID)
  echo "snort-docker-id: $FULL_SNORT_DOCKER_ID"
  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -t nat -A PREROUTING -d $ROUTER_IP -s $CLIENT1_IP -j DNAT --to-destination $SERVER1_IP
  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -t nat -I POSTROUTING -s $CLIENT1_IP -j SNAT --to-source $ROUTER_IP
  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -A FORWARD -d $SERVER1_IP -i eth0 -j ACCEPT

  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -t nat -A PREROUTING -d $ROUTER_IP -s $CLIENT2_IP -j DNAT --to-destination $SERVER2_IP
  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -t nat -I POSTROUTING -s $CLIENT2_IP -j SNAT --to-source $ROUTER_IP
  sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -A FORWARD -d $SERVER2_IP -i eth0 -j ACCEPT

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
