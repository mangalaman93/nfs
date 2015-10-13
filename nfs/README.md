# Test Locally
*`docker run --rm -it --net=host --volume=/:/rootfs:ro --volume=/var/run:/var/run:rw --volume=/sys:/sys:ro --volume=/var/lib/docker/:/var/lib/docker:ro --name=cadvisor mangalaman93/cadvisor -storage_driver=influxdb -storage_driver_user=cadvisor -storage_driver_password=cadvisor -storage_driver_host=0.0.0.0:8086`
