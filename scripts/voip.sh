#!/bin/bash

if [ "$#" -lt 1 ]; then
  echo "error!"
  echo "Usage: $0 start|stop" >&2
  exit 1
fi

# hosts
CUR_HOST=jedi054
OTH_HOST=jedi055
OVS_BRIDGE=br-int

# default options
IS_SNORT=true
HOST_SIPP_CLIENT=$CUR_HOST
HOST_SIPP_SERVER=$CUR_HOST
HOST_SNORT=$CUR_HOST
NUM_SIPP=3

# get openstack environment variables
source ~/devstack/openrc admin
# influxdb config
source ~/nfs/.influxdb.config
# maximum limit on read buffer size
sudo sysctl -w net.core.rmem_max=26214400
BUFF_SIZE=1048576

set_controller() {
  sudo ovs-vsctl set-controller ${OVS_BRIDGE} "tcp:${CUR_HOST}:6633"
}

# finds ip address given id of nova instance! $1 -> id
# ip address is stored in OUT_IP variable after return
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
  nova delete snort &>/dev/null
  for (( i = 0; i < $NUM_SIPP; i++ )); do
    nova delete "sipp-client$i" "sipp-server$i" &>/dev/null
  done

  echo "delete volumes yourselves!!!"
  echo "cleanup done!"
}

# ensure nova instance is running on given host! $1 -> id, $2 -> host
check_host() {
  THIS_HOST=$(nova show $1 | grep "OS-EXT-SRV-ATTR:hypervisor_hostname" | awk '{print $4}')
  if [[ $THIS_HOST != $2 ]]; then
    echo "Error: $1 started on unexpected host $THIS_HOST instead of $2!"
    stop_exp
    exit 1
  fi
}

# throw error if a nova instance is running! $1 -> id
err_if_running() {
  nova show $1 &>/dev/null
  if [[ $? -eq 0 ]]; then
    echo "Error: $1 is already running!"
    exit 1
  fi
}

# throw error if a nova instance is not running! $1 -> id
err_if_not_running() {
  nova show $1 &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: $1 did not run!"
    stop_exp
    exit 1
  fi
}

set_routes() {
  if [[ "$IS_SNORT" != "true" ]]; then
    return
  fi

  THIS_HOST=$(hostname)
  for (( i = 0; i < $NUM_SIPP; i++ )); do
    find_ip "sipp-server$i"
    SERVER_IP[$i]=$OUT_IP
  done
  find_ip snort
  ROUTER_IP=$OUT_IP

  echo "setting up routes"
  if [[ "$HOST_SIPP_CLIENT" == "$THIS_HOST" ]]; then
    for (( i = 0; i < $NUM_SIPP; i++ )); do
      CLIENT_ID=$(nova show sipp-client$i | grep "| id" | awk '{print $4}')
      FULL_CLIENT_DOCKER_ID=$(docker inspect --format '{{ .Id }}' sipp-client$i-$CLIENT_ID)
      sudo ip netns exec $FULL_CLIENT_DOCKER_ID ip route add ${SERVER_IP[$i]} via $ROUTER_IP
    done
  fi

  if [[ "$HOST_SNORT" == "$THIS_HOST" ]]; then
    SNORT_ID=$(nova show snort | grep "| id" | awk '{print $4}')
    FULL_SNORT_DOCKER_ID=$(docker inspect --format '{{ .Id }}' snort-$SNORT_ID)
    for (( i = 0; i < $NUM_SIPP; i++ )); do
      find_ip "sipp-client$i"
      CLIENT_IP=$OUT_IP
      sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -t nat -A PREROUTING -d $ROUTER_IP -s $CLIENT_IP -j DNAT --to-destination ${SERVER_IP[$i]}
      sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -t nat -I POSTROUTING -s $CLIENT_IP -j SNAT --to-source $ROUTER_IP
      sudo ip netns exec $FULL_SNORT_DOCKER_ID iptables -A FORWARD -d ${SERVER_IP[$i]} -i eth0 -j ACCEPT
    done
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

  for (( i = 0; i < $NUM_SIPP; i++ )); do
    err_if_running "sipp-server$i"
    err_if_running "sipp-client$i"
  done
  err_if_running snort

  # run monsipp
  cd ~/nfs/ && sudo ./monsipp -d $INFLUXDB_IP:$INFLUXDB_PORT:$INFLUXDB_USER:$INFLUXDB_PASS

  # wait for monsipp to start on other server
  echo -n "run the same script on $OTH_HOST and press enter"
  read text

  # run sipp servers
  for (( i = 0; i < $NUM_SIPP; i++ )); do
    echo "running sipp-server$i"
    nova boot --image mangalaman93/sipp --meta ARGS="-buff_size $BUFF_SIZE -sn uas" --availability-zone regionOne:$HOST_SIPP_SERVER --flavor c1.tiny "sipp-server$i" > /dev/null
    sleep 3
    check_host "sipp-server$i" $HOST_SIPP_SERVER
    find_ip "sipp-server$i"
    SERVER_IP[$i]=$OUT_IP
    echo "server$i: ${SERVER_IP[$i]}"
  done

  # run snort
  if [[ "$IS_SNORT" == "true" ]]; then
    echo "running snort"
    nova boot --image mangalaman93/snort --meta OPT_CAP_ADD=NET_ADMIN --availability-zone regionOne:$HOST_SNORT --flavor c1.small snort > /dev/null
    sleep 3
    check_host snort $HOST_SNORT
    find_ip snort
    ROUTER_IP=$OUT_IP
    echo "router: $ROUTER_IP"
  fi

  # run sipp-clients
  for (( i = 0; i < $NUM_SIPP; i++ )); do
    echo "running sipp-client$i"
    nova boot --image mangalaman93/sipp --meta ARGS="-buff_size $BUFF_SIZE -sn uac ${SERVER_IP[$i]}:5060" --availability-zone regionOne:$HOST_SIPP_CLIENT --flavor c1.tiny "sipp-client$i" > /dev/null
    sleep 3
    check_host "sipp-client$i" $HOST_SIPP_CLIENT
    find_ip "sipp-client$i"
    CLIENT_IP=$OUT_IP
    echo "client$i: $CLIENT_IP"
  done

  # run cadvisor
  docker run -d --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro --name=$CUR_HOST-cadvisor mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_user=$INFLUXDB_USER -storage_driver_password=$INFLUXDB_PASS -storage_driver_host=$INFLUXDB_IP:$INFLUXDB_PORT > /dev/null

  echo "running checks..."
  docker inspect $CUR_HOST-cadvisor &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: cadvisor did not run!"
    stop_exp
    exit 1
  fi

  pidof monsipp &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: monsipp did not run!"
    stop_exp
    exit 1
  fi

  for (( i = 0; i < $NUM_SIPP; i++ )); do
    err_if_not_running "sipp-server$i"
    err_if_not_running "sipp-client$i"
  done

  if [[ "$IS_SNORT" == "true" ]]; then
    err_if_not_running snort
  fi

  set_routes
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

  for (( i = 0; i < $NUM_SIPP; i++ )); do
    err_if_not_running "sipp-server$i"
    err_if_not_running "sipp-client$i"
  done

  if [[ "$IS_SNORT" == "true" ]]; then
    err_if_not_running snort
  fi

  # run cadvisor
  docker run -d --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro --name=$OTH_HOST-cadvisor mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_user=$INFLUXDB_USER -storage_driver_password=$INFLUXDB_PASS -storage_driver_host=$INFLUXDB_IP:$INFLUXDB_PORT > /dev/null

  echo "running checks..."
  docker inspect $OTH_HOST-cadvisor &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: cadvisor did not run!"
    stop_exp
    exit 1
  fi

  pidof monsipp &>/dev/null
  if [[ $? -ne 0 ]]; then
    echo "Error: monsipp did not run!"
    stop_exp
    exit 1
  fi

  set_routes
}

case $1 in
  "start")
    if [ "$#" -gt 1 ]; then
      if [ "$#" -lt 5 ]; then
        echo "error!"
        echo "Usage: $0 start host_[client server snort(false)] [num_sipp]" >&2
        exit 1
      else
        HOST_SIPP_CLIENT=$2
        HOST_SIPP_SERVER=$3
        if [[ "$4" == "false" ]]; then
          IS_SNORT=false
        else
          IS_SNORT=true
          HOST_SNORT=$4
        fi
        NUM_SIPP=$5
      fi
    fi
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
    if [ "$#" -gt 1 ]; then
      if [ "$#" -gt 2 ]; then
        echo "error!"
        echo "Usage: $0 stop [num_sipp]" >&2
        exit 1
      else
        NUM_SIPP=$2
      fi
    fi
    stop_exp
    ;;
  *)
    echo "Error: Unknown command!"
    exit 1
    ;;
esac
