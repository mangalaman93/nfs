#!/bin/bash

# influxdb
INFLUXDB_IP=
INFLUXDB_PORT=
INFLUXDB_USER=
INFLUXDB_PASS=
INFLUXDB_DB=cadvisor

# without snort
container="sipp-server1-b309cf67-e824-4914-b07c-00964766ce59"
from=1443417045727
to=1443418189982

ruby idb.rb -db $INFLUXDB_DB -from $from -to $to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series call_rate -cont $container
ruby idb.rb -db $INFLUXDB_DB -from $from -to $to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series retransmissions -cont $container
ruby idb.rb -db $INFLUXDB_DB -from $from -to $to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series response_time -cont $container
ruby idb.rb -db $INFLUXDB_DB -from $from -to $to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series failed_calls -cont $container

# wait
read text

# with snort
container="sipp-server1-ad699f32-97dd-4468-b0f2-9f143bed40cf"
from=1443415206518
to=1443416529796

ruby idb.rb -db $INFLUXDB_DB -from $from -to $to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series call_rate -cont $container
ruby idb.rb -db $INFLUXDB_DB -from $from -to $to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series retransmissions -cont $container
ruby idb.rb -db $INFLUXDB_DB -from $from -to $to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series response_time -cont $container
ruby idb.rb -db $INFLUXDB_DB -from $from -to $to -ip $INFLUXDB_IP -user $INFLUXDB_USER -pass $INFLUXDB_PASS -series failed_calls -cont $container
