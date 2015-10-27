#!/bin/bash
if [ -z "$ARGS" ]; then
    echo "ERROR: \"ARGS\" env var not set!"
    exit 1
fi

# wait for networking to setup
sleep 5

ifaces=$(ifconfig | grep "inet " | awk -F'[: ]+' '{print $4}')
for iface in $ifaces; do
	if [ "$iface" != "127.0.0.1" ]; then
		IF=$iface
	fi
done
if [[ -z $IF ]]; then
	echo "ERROR: a valid interface is not found!"
	exit 1
fi

cd /data && sipp -bg -trace_stat -fd 5s -trace_rtt -rtt_freq 1000 -trace_logs -trace_err -i $IF $ARGS
tail -f /dev/null
