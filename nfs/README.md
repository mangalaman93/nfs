# Test Locally
* Compile code `make`
* Run nfs `./build/nfs`
* Run cadvisor `docker run --rm -it --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro --name=cadvisor mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_user=root -storage_driver_password=root -storage_driver_host=0.0.0.0:8086`
