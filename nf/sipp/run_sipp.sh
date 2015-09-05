#!/bin/sh
if [ -z "$ARGS" ]; then
    echo "ERROR: \"ARGS\" env var not set!"
    exit 1
fi

cd /data && sipp -bg -trace_stat -fd 1s -trace_rtt -rtt_freq 1 $ARGS
tail -f /dev/null
