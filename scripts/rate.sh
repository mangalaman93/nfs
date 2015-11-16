#!/bin/bash

# checking command line arguments
if [ "$#" -lt 4 ]; then
  echo "error!"
  echo "Usage: $0 <rate> <prefix> <ip-address-min> <ip-address-max>" >&2
  exit 1
fi

# command line arguments
PORT=8888

# $1 -> ip, $2 -> rate
set_rate() {
  bash -c "echo \"cset rate $2\" >/dev/udp/$1/$PORT"
  echo "set rate to $2 for $1:$PORT"
}

# $1 -> ip -> rate
stop() {
  bash -c "echo \"q\" >/dev/udp/$1/$PORT"
  echo "stopping client"
}

for i in $(seq $3 $4); do
  sudo iptables -F;
  set_rate $2.$i $1;
done
