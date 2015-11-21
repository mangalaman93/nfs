#!/bin/bash

## This scripts performs a double ssh and downloads .table
## and .logs files corresponding to the experiment from the
## folder /opt/nfs/stack from both the physical servers
#

if [ "$#" -lt 1 ]; then
  echo "error!"
  echo "Usage: $0 <folder>" >&2
  exit 1
fi

USER=amangal7
JUMP_SERVER=jedi-gateway
DISTANT_SERVERS="10.1.21.13 10.1.21.14"
for ip in $DISTANT_SERVERS; do
	echo "please run in separate terminal and press enter"
	echo -n "ssh -L 6000:$ip:22 ${USER}@${JUMP_SERVER}"
	read text
	scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P 6000 ${USER}@localhost:/opt/stack/nfs/*.table $1/
	scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -P 6000 ${USER}@localhost:/opt/stack/nfs/*.log $1/sipp-$ip.log
done
