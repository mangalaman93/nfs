#!/bin/bash

# checking command line arguments
if [ "$#" -lt 3 ]; then
  echo "error!"
  echo "Usage: $0 <rate> <ip-address-min> <ip-address-max> (assuming prefix 10.0.1.*)" >&2
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

for i in $(seq $2 $3); do
  sudo iptables -F;
  set_rate 10.0.1.$i $1;
done
