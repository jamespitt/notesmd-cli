#!/bin/bash
go build -o notesmd-cli .
systemctl stop --user notesmd-cli.service
lsof /usr/local/bin/notesmd-cli
sudo cp notesmd-cli /usr/local/bin/
systemctl start --user notesmd-cli.service
