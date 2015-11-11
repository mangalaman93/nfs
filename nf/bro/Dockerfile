FROM ubuntu:latest
MAINTAINER = Aman Mangal <amanmangal@gatech.edu>

# parameters
ENV BRO_VERSION bro-2.4.1

# install depedencies from standard repos
RUN apt-get update && \
    apt-get install -y bison \
                       build-essential \
                       cmake \
                       flex \
                       gcc \
                       g++ \
                       python-dev \
                       libgeoip-dev \
                       libmagic-dev \
                       libpcap-dev \
                       libssl-dev \
                       swig2.0 \
                       make \
                       wget \
                       zlib1g-dev

# install bro
RUN wget http://www.bro.org/downloads/release/$BRO_VERSION.tar.gz && \
    tar -xzf $BRO_VERSION.tar.gz && \
    cd $BRO_VERSION && \
    ./configure && \
    make && \
    make install && \
    cd ../ && rm -r $BRO_VERSION $BRO_VERSION.tar.gz
