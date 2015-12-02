# Test Moncont locally
* influxdb `docker run --rm -it -p 8083:8083 -p 8086:8086 -e PRE_CREATE_DB="cadvisor" tutum/influxdb`
* moncont `go build && sudo ./moncont 0.0.0.0:8086` OR `docker build --rm -t mangalaman93/moncont . && docker run --rm -it --net=host -v /proc:/host/proc:ro -v /var/run:/var/run:ro -v /var/lib/docker/:/var/lib/docker:ro mangalaman93/moncont 0.0.0.0:8086`
* sipp server `docker run -d -e ARGS="-sn uas" --name "sipp-server" mangalaman93/sipp`
* snort `docker run -d --cap-add=NET_ADMIN --name "snort" mangalaman93/snort`
* sipp client ``docker run -d -e ARGS="-sn uac `docker inspect --format '{{ .NetworkSettings.IPAddress }}' sipp-server`" --name "sipp-client" mangalaman93/sipp``
* delete all the containers `docker kill $(docker ps -aq) && docker rm $(docker ps -aq)`
* delete all the volumes `docker volume rm $(docker volume ls -q)`
* delete untagged images `docker images | grep "^<none>" | awk "{print $3}"`

# TODO
* moncont consumes large amount of CPU
