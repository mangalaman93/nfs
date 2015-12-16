## Helpful Commands
* influxdb `docker run --rm -it -p 8083:8083 -p 8086:8086 -v /opt/stack/influxdb/:/data --name influxdb -e PRE_CREATE_DB="cadvisor" tutum/influxdb`
* grafana `docker run --rm -it -p 3000:3000 --name grafana-server -v /opt/stack/grafana:/var/lib/grafana/ grafana/grafana`
* cadvisor ui `http://localhost:8080/containers/system.slice`
* nfs `./nfs -c /opt/stack/nfs/.voip.conf`
* load `echo "cset rate 10" > /dev/udp/173.16.1.4/8888`
* delete all the containers `docker kill $(docker ps -aq) && docker rm $(docker ps -aq)`
* delete all the volumes `docker volume rm $(docker volume ls -q)`
* delete untagged images `docker images | grep "^<none>" | awk "{{print $3}}"`

## Note
* Make sure to use docker binary from [here](https://github.com/mangalaman93/docker/raw/merge_add_set/bundles/1.9.0/binary/docker-1.9.0)
