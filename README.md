gbox is a project designed to run the Helix Bridge Relayer (https://github.com/helix-bridge/relayer) in a Docker container. The password is set via HTTP POST, and gbox will start the relayer and automatically input the password. The password is only stored in memory.

## Features:

* Start and manage Helix Bridge Relayer container
* Set password via HTTP API
* Monitor the relayer's running status
* Automatic restart after configuration changes

## Configuration

gbox uses environment variables for configuration. Here are the details:

### Runner Configuration

* SERVER_ADDR (Default: :8080): Server listening address
* FETCH_CONFIG_URL (Default: None): Configuration file fetch URL. If empty, local configuration files will be used, and configuration changes will not automatically restart the container.
* CHECK_INTERVAL (Default: 1m): Interval for checking the running status of the relayer, e.g. 1s, 1m, or 1h

### Helix Configuration

* HELIX_ENV (Default: LP_BRIDGE_PATH=./.maintain/configure.json,LP_BRIDGE_STORE_PATH=./.maintain/db): Helix relayer environment variables, set in comma-separated key-value pairs, e.g. env1=true,env2=false
* HELIX_COMMAND (Default: node ./dist/src/main): Command to start the relayer
* HELIX_ROOT_DIR (Default: ./relayer): Helix relayer code root directory
* CONFIG_PATH (Default: ./.maintain/configure.json): Configuration file path (relative to HELIX_ROOT_DIR)

## Running

Run gbox with the following Docker command:
```Bash
docker run -dt --restart=always --name=helix-relayer \
    -e SERVER_ADDR=:8080 \
    -e FETCH_CONFIG_URL= \  # Optional, configuration fetch URL
    -e LP_BRIDGE_STORE_PATH=/data/db \  # Optional, override default value
    -e LP_BRIDGE_PATH=/data/configure.json \  # Optional, override default value
    -v ~/relayer/.maintain:/data \  # Optional, mount external storage (optional)
    perrorone/helix-relayer
```

Enter the container (optional):
```Bash
docker exec -it helix-relayer /bin/sh
```

Set password:
```Bash
curl -X POST "http://127.0.0.1:8080/pass" \
-H "Content-Type: application/json" \
-d '{"p": "<your-password>"}'
```

**Note: Replace <your-password> with your actual password.**
