#!/usr/bin/python

import sys
from pyretic.lib.corelib import *
from pyretic.lib.std import *

def main():
    if len(sys.argv) < 4:
    	print "usage: <mac-snort> <ip-servers>"
        raise "Need server IPs as arguments"
    snort_mac = sys.argv[2]
    servers = sys.argv[3:]

    policy = None
    for s in servers:
    	m = (match(dstip=IPAddr(s)) >> modify(dstmac=EthAddr(snort_mac)))
    	if policy:
        	policy = policy + m
        else:
        	policy = m
    return policy
