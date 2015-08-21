#!/bin/sh

# checking command line arguments
if [ "$#" -lt 1 ]; then
  echo "error!"
  echo "Usage: $0 <ip-address1> <ip-address2> ..." >&2
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

# following scenario is emulated here
#           <---20s--> <---20s--> <---20s-->
#    2000              __________
#                     |          |
#    1000   __________|          |__________
#          |                                |
#     0  __|                                |__
#
for ip in "$@"; do
  set_rate $ip 1000
done
sleep 30s

for ip in "$@"; do
  set_rate $ip 2000
done
sleep 30s

for ip in "$@"; do
  set_rate $ip 1000
done
sleep 30s

for ip in "$@"; do
  stop $ip
done
