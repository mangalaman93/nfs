FROM ubuntu:latest
MAINTAINER = Aman Mangal <amanmangal@gatech.edu>

# install depedencies from standard repos
RUN apt-get update && apt-get install -y build-essential wget libncurses5-dev libpcap-dev libdnet-dev

# install sipp
RUN wget http://iweb.dl.sourceforge.net/project/sipp/sipp/3.4/sipp-3.3.990.tar.gz && tar -xf sipp-3.3.990.tar.gz && cd sipp-3.3.990 && ./configure --with-pcap && make && make install

# copy scenarios
COPY scens/* /scens/
COPY run_sipp.sh /

# command to run sipp
CMD "/bin/sh" "/run_sipp.sh"

# clean up
RUN rm -r sipp-3.3.990.tar.gz sipp-3.3.990/
