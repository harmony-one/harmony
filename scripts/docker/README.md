## Docker Image

Included in this repo is a Dockerfile that you can launch harmony node for trying it out. Docker images are available on `harmonyone/harmony`.

You can build the docker image with the following commands:
```bash
make docker
```

If your build machine has an ARM-based chip, like Apple silicon (M1), the image is built for `linux/arm64` by default. To build for `x86_64`, apply the --platform arg:

```bash
docker build --platform linux/amd64 -t harmonyone/harmony -f Dockerfile .
```

Before start the docker, dump the default config `harmony.conf` by running:

for testnet
```bash
docker run -v $(pwd)/config:/harmony --rm --name harmony harmonyone/harmony harmony config dump --network testnet harmony.conf
```
for mainnet
```bash
docker run -v $(pwd)/config:/harmony --rm --name harmony harmonyone/harmony harmony config dump harmony.conf
```

make your customization. `harmony.conf` should be mounted into default `HOME` directory `/harmony` inside the container. Assume `harmony.conf`, `blskeys` and `hmykey` are under `./config` in your current working directory, you can start your docker container with the following command:
```bash
docker run -v $(pwd)/config:/harmony --rm --name harmony -it harmonyone/harmony
```

If you need to open another shell, just do:
```bash
docker exec -it harmonyone/harmony /bin/bash
```

We also provide a `docker-compose` file for local testing

To use the container in kubernetes, you can use a configmap or secret to mount the `harmony.conf` into the container
```bash
containers:
  - name: harmony
    image: harmonyone/harmony
    ports:
      - name: p2p
        containerPort: 9000  
      - name: rpc
        containerPort: 9500
      - name: ws
        containerPort: 9800     
    volumeMounts:
      - name: config
        mountPath: /harmony/harmony.conf
  volumes:
    - name: config
      configMap:
        name: cm-harmony-config
    
```

Your configmap `cm-harmony-config` should look like this:
```
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-harmony-config
data:
  harmony.conf: |
    ...
```