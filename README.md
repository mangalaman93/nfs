# nfs
Network Function Scheduler

# Test NFS locally
* influxdb `docker run --rm -it -p 8083:8083 -p 8086:8086 -e PRE_CREATE_DB="cadvisor" tutum/influxdb`
* nfs `./nfs -c /opt/stack/nfs/.voip.conf`
* examples `go build && ./examples`

# TODO
* client doesn't throw error when socket is closed
* fix algorithm
* Update ndpi, bro network functions (NF)
* Use explicit queues for NFs
