FROM progrium/busybox
MAINTAINER amanmangal@gatech.edu

# Grab cadvisor from the staging directory.
ADD cadvisor /usr/bin/cadvisor

EXPOSE 8080
ENTRYPOINT ["/usr/bin/cadvisor"]

