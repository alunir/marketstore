
```
alias docker-compose='docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$(pwd):$(pwd)" \
  -w "$(pwd)" \
  docker compose'
```

 1. ```sudo mkfs.xfs -f /dev/sdb```
 2. ```sudo mount -t xfs -o noatime,logbufs=8 /dev/sdb /home/nsplat/marketstore/data```