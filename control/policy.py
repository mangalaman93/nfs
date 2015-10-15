#!/usr/bin/python

import sys
from pyretic.lib.corelib import *
from pyretic.lib.std import *

def main():
    if len(sys.argv) < 3:
        raise "Need server IPs as arguments"
    servers = sys.argv[2:]

    policy = None
    for s in servers:
        policy = policy + (match(dstip=s) >> fwd())
    return policy
