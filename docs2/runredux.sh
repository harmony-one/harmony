swagger generate spec -w ../cmd/harmony/ -o swagger.json
swagger serve swagger.json --port 8082 --host=0.0.0.0 --no-open
