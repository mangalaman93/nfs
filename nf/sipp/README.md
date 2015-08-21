# build sipp container
```
docker build --rm -t mangalaman93/sipp nf/sipp/
```

# run sipp server
```
export SCEN=<scenario>
docker run --rm -it -p 5060:5060/udp --name "sipp-server" mangalaman93/sipp sipp -sf /scens/$SCEN
```

# run sipp client
```
export SIPP_SERVER=<server-ip>
export SCEN=<scenario>
docker run --rm -it --name "sipp-client" -p 8888:8888/udp mangalaman93/sipp sipp -sf /scens/$SCEN $SIPP_SERVER:5060
```
