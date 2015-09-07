#!/bin/sh
if [ -z "$ARGS" ]; then
    echo "ERROR: \"ARGS\" env var not set!"
    exit 1
fi

# wait for networking to setup
sleep 5

IF=$(ifconfig | grep "inet " | awk -F'[: ]+' '{print $4}')
cd /data && sipp -bg -trace_stat -fd 1s -trace_rtt -rtt_freq 1 -trace_logs -trace_err -i $IF $ARGS
tail -f /dev/null
