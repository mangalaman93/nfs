#!/bin/bash

SHARE_INIT=12
SHARE_STEP_SIZE=12
SHARE_MAX=90
RATE_INIT=250
RATE_STEP_SIZE=250
RATE_MAX=2500
CPU_PERIOD=100000
EXP_DURATION=140
FOLDER=exp

# get openstack environment variables
source ~/devstack/openrc admin

for (( shares = $SHARE_INIT; shares <= $SHARE_MAX; shares+=$SHARE_STEP_SIZE )); do
  for (( rate = $RATE_INIT; rate <= $RATE_MAX; rate+=$RATE_STEP_SIZE )); do
    # clean up
    sudo rm -r /var/lib/docker/volumes

    shares_val=$(($shares*1024/100))
    quota=$(($shares_val * $CPU_PERIOD / 1024))
    echo "running experiment with shares=$shares%, quota=$quota, rate=$rate"
    echo "\n" | ./voip start jedi054 jedi054 jedi054 1 1

    snort_id=$(docker ps | grep snort | awk '{print $1}')
    docker set --cpu-shares $shares_val --cpu-quota $quota $snort_id

    client_ip=$(nova show sipp-client0 | grep network | tr -d [:space:] | awk -F'[,|]' '{print $3}')
    if [[ $client_ip == *":"* ]]; then
      client_ip=$(nova show sipp-client0 | grep network | tr -d [:space:] | awk -F'[,|]' '{print $4}')
    fi
    sudo iptables -F
    ping -c 2 $client_ip
    bash -c "echo \"cset rate $rate\" >/dev/udp/$client_ip/8888"
    echo "set rate to $rate for $client_ip"

    sleep $EXP_DURATION
    ./voip stop 1 1

    # copy files
    to_folder=$FOLDER/$shares-$rate
    mkdir -p $to_folder
    sudo mv contmon.log $to_folder/
    sudo cp -r /var/lib/docker/volumes $to_folder/

    sleep 20
  done
done
