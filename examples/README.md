## Test locally
* influxdb `docker run --rm -it -p 8083:8083 -p 8086:8086 --name influxdb -e PRE_CREATE_DB="cadvisor" tutum/influxdb`
* grafana `docker run --rm -it -p 3000:3000 --name grafana-server -v /home/ubuntu/grafana:/var/lib/grafana/ grafana/grafana`
* cadvisor ui `http://localhost:8080/containers/system.slice`
* nfs `./nfs -c /home/ubuntu/nfs/.voip.conf`
* load `echo "cset rate 10" > /dev/udp/173.16.1.4/8888`

## Note
* Make sure to use docker binary from [here](https://github.com/mangalaman93/docker/raw/merge_add_set/bundles/1.9.0/binary/docker-1.9.0)

## ovs commands
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
