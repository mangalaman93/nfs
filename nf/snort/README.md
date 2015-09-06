# Run Snort
```
docker build --rm -t mangalaman93/snort nf/snort/
docker run --rm --cap-add=NET_ADMIN --name snort mangalaman93/snort
docker kill snort
```

# Testing with Snort
```
gorun nf/rtplot.go snort 1000
```

# Reference
* [Snort3 html Manual](https://www.snort.org/downloads/#snort-3.0)
