# build pyretic container
```
docker build --rm -t mangalaman93/pyretic routing/
```

# Run Pyretic
```
docker run --rm -it -p 6633:6633 mangalaman93/pyretic pyretic.examples.policy [SERVER_IPs]
```
