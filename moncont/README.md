# Test Moncont locally
* influxdb `docker run --rm -it -p 8083:8083 -p 8086:8086 -e PRE_CREATE_DB="cadvisor" tutum/influxdb`
* monsipp `go build && sudo ./moncont 0.0.0.0:8086`
* sipp server `docker run -d -e ARGS="-sn uas" --name "sipp-server" mangalaman93/sipp`
* snort `docker run -d --cap-add=NET_ADMIN --name "snort" mangalaman93/snort`
* sipp client ``docker run -d -e ARGS="-sn uac `docker inspect --format '{{ .NetworkSettings.IPAddress }}' sipp-server`" --name "sipp-client" mangalaman93/sipp``
* delete all the containers `docker kill $(docker ps -aq) && docker rm $(docker ps -aq)`
* delete all the volumes `docker volume rm $(docker volume ls -q)`

# TODO
* moncont consumes large amount of CPU
* improve/optimize influxdb api
