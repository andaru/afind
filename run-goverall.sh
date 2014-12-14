#!/bin/bash

token="E58ALNT0xWfennrfAgs2Fgkvq1nAR8hD4"
goveralls=${HOME}/gopath/bin/goveralls

echo "Testing afind"
go test github.com/andaru/afind/...

echo "Running coverage reports"
${goveralls} -repotoken ${token} github.com/andaru/afind/afind -- -covermode=count
