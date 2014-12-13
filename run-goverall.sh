#!/bin/bash

token="E58ALNT0xWfennrfAgs2Fgkvq1nAR8hD4"
goveralls=${HOME}/gopath/bin/goveralls
go=${HOME}/gopath/bin/go
pfx=github.com/andaru/afind

echo "Testing afind"
${go} test github.com/andaru/afind/...

echo "Running coverage reports"
${goveralls} -repotoken ${token} ${pfx}/afind ${pfx}/afind/api ${pfx}/errs -- -covermode=count
