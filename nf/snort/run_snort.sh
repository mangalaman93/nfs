#!/bin/sh
export snort_path=/usr/local
export LUA_PATH=$snort_path/include/snort/lua/\?.lua\;\;
export SNORT_LUA_PATH=$snort_path/etc/snort

iptables -A FORWARD -j NFQUEUE
snort -c $snort_path/etc/snort/snort.lua -R $snort_path/etc/snort/sample.rules --max-packet-threads 8 --daq nfq -Q -L dump -l /log/
