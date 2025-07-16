#!/bin/bash

set -e

cd "$(dirname "$0")"

# Build poweroutg-mon binary for Linux amd64 into ../bin/
echo "[*] Building binary into ../bin/poweroutg-mon"
GOOS=linux GOARCH=amd64 go build -o ../bin/poweroutg-mon ../main.go

# Build RPM package using fpm, referencing built binary in bin/
echo "[*] Building rpm package with fpm"
fpm \
  -s dir \
  -t rpm \
  -n poweroutg-mon \
  -v 1.0.0 \
  --description "Power Outage Connectivity Monitor Agent" \
  --url "https://github.com/your-org/your-repo" \
  --maintainer "dat <trduydat@gmail.com>" \
  --after-install ../postinst.sh \
  ../bin/poweroutg-mon=/usr/local/bin/poweroutg-mon \
  ../poweroutg-mon.service=/etc/systemd/system/poweroutg-mon.service

echo "[+] Build finished"