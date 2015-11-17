# Test locally
* cadvisor `docker run -d --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_host=0.0.0.0:8087`
* delete all the containers `docker kill $(docker ps -aq) && docker rm $(docker ps -aq)`
* ovs commands:
```
sudo ovs-vsctl add-br ovsbr
sudo ifconfig ovsbr 173.16.1.1 netmask 255.255.255.0 up
docker run --rm -it --net=none --name=sipp-server mangalaman93/sipp bash
sudo ovs-docker add-port ovsbr eth0 sipp-server --ipaddress=173.16.1.2/24
sipp -sn uas -i 173.16.1.2
docker run --rm -it --net=none --name=sipp-client mangalaman93/sipp bash
sudo ovs-docker add-port ovsbr eth0 sipp-client --ipaddress=173.16.1.3/24
sipp -sn uac -i 173.16.1.3 173.16.1.2:5060
sudo ovs-docker del-port ovsbr eth0 sipp-server
sudo ovs-docker del-port ovsbr eth0 sipp-client
sudo ovs-vsctl del-br ovsbr
```
* influxdb `docker run -d -p 8083:8083 -p 8086:8086 -e PRE_CREATE_DB="cadvisor" tutum/influxdb`
