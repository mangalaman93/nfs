# build sipp container
```
docker build --rm -t mangalaman93/sipp nf/sipp/
```

# run sipp server
```
docker run --rm -it -e CMD="sipp -sf /scens/<scenario>" -p 5060:5060/udp --name "sipp-server" mangalaman93/sipp
```

# run sipp client
```
docker run --rm -it -e CMD="sipp -sf /scens/<scenario> <server-ip>:5060" --name "sipp-client" -p 8888:8888/udp mangalaman93/sipp
```
