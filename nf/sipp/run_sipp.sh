#!/bin/sh

if [ -z "$CMD" ]; then
    echo "ERROR: \"CMD\" env var not set!"
    exit 1
fi

eval "$CMD"
