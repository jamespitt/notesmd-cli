#!/bin/bash
go build -o notesmd-cli .
systemctl stop --user notesmd-cli.service
sudo cp notesmd-cli /usr/local/bin/
lsof /user/local/bin/notesmd-cli
systemctl start --user notesmd-cli.service
