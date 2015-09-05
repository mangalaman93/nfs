# build sipp container
```
docker build --rm -t mangalaman93/sipp nf/sipp/
```

# run sipp server
```
docker run --rm -d -e ARGS="-sf /scens/<scenario>" --name "sipp-server" mangalaman93/sipp
```

# run sipp client
```
docker run --rm -d -e ARGS="-sf /scens/<scenario> <server-ip>:<port>" --name "sipp-client" mangalaman93/sipp
```
