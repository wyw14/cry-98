# CryoSafe

CryoSafe coordinates liquid-nitrogen vessels, probes, fill circuits, valves,
pressure settling, alarms, and local recovery for a cryogenic storage area.

Run the service with `go run ./cmd/cryosafe`. It listens on
`127.0.0.1:19698` by default. The data directory can be changed with
`CRYOSAFE_DATA_DIR`, and the listen address with `CRYOSAFE_ADDR`.
