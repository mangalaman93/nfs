# Run Suricata
```
docker build --rm -t mangalaman93/suricata nf/suricata/
docker run --rm -it --cap-add=NET_ADMIN --name suricata mangalaman93/suricata
```
