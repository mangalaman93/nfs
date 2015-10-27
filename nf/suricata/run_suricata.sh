#!/bin/sh
iptables -A FORWARD -j NFQUEUE
suricata -q 0
