# nfs
Network Function Scheduler

# nfh
Network Function Host

# setup nfh
```
sudo apt-get update
sudo apt-get install -y build-essential collectd docker.io golang-go
sudo usermod -aG docker ${USER}
mkdir $HOME/gocode
echo "GOPATH=\"$HOME/gocode\"" | sudo tee -a /etc/environment
```
>> reboot the machine now

```
sudo mkdir /var/log/collectd/
go get github.com/mangalaman93/nfs
cd collectd && make && sudo make install && cd ../
sudo service collectd restart
```
>> Make sure to setup `/etc/collectd/collectd.conf` before starting collectd

# TODO
* Influxdb setup, retrieve data securely
* Optimize data sent by collectd over network
* Network functions
* Command (from NFS to NFH) syntax
* Simple Algorithm
* Snort load test
* Parse usage from NFH to NFS (go-collectd)

# References
* [Run system commands in go](http://www.darrencoxall.com/golang/executing-commands-in-go/)
