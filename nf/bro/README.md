# Run Bro
```
docker build --rm -t mangalaman93/bro nf/bro/
docker run --rm -it --cap-add=NET_ADMIN --name bro mangalaman93/bro
```
