# Test Monsipp locally
* influxdb `docker run -d -p 8083:8083 -p 8086:8086 -e PRE_CREATE_DB="cadvisor" tutum/influxdb`
* monsipp `sudo ./build/monsipp 0.0.0.0:8086`
* sipp server `docker run -d -e ARGS="-sn uas" --name "sipp-server" mangalaman93/sipp`
* sipp client ``docker run -d -e ARGS="-sn uac `docker inspect --format '{{ .NetworkSettings.IPAddress }}' sipp-server`" --name "sipp-client" mangalaman93/sipp``
* delete all the containers `docker kill $(docker ps -aq) && docker rm $(docker ps -aq)`
* delete all the volumes
