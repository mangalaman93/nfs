#!/bin/bash

## This script downloads influxdb data using idb.rb script.
## It creates two files containing data for the two cases
## depending upon whether snort was running. It needs to be
## provided with container id and points of time when the
## experiments were performed
#

if [ "$#" -lt 6 ]; then
  echo "error!"
  echo "Usage: $0 [with snort]<container> <from> <to> [without]<container> <from> <to>" >&2
  exit 1
fi

# influxdb
source .influxdb.config
INFLUXDB_DB=cadvisor

# without snort
with_container=$1
with_from=$2
with_to=$3
without_container=$4
without_from=$5
without_to=$6

for metric in "call_rate" "retransmissions" "response_time" "failed_calls"; do
	ruby idb.rb -db $INFLUXDB_DB -from $with_from -to $with_to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series $metric -cont $with_container
done
paste call_rate.txt retransmissions.txt response_time.txt failed_calls.txt > with_snort.col

for metric in "call_rate" "retransmissions" "response_time" "failed_calls"; do
	ruby idb.rb -db $INFLUXDB_DB -from $without_from -to $without_to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series $metric -cont $without_container
done
paste call_rate.txt retransmissions.txt response_time.txt failed_calls.txt > without_snort.col

rm *.txt
