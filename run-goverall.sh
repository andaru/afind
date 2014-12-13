#!/bin/bash

token="E58ALNT0xWfennrfAgs2Fgkvq1nAR8hD4"
goveralls=${HOME}/gopath/bin/goveralls

${goveralls} -repotoken ${token} github.com/andaru/afind/afind -- -covermode=count
${goveralls} -repotoken ${token} github.com/andaru/afind/api -- -covermode=count
${goveralls} -repotoken ${token} github.com/andaru/afind/errs -- -covermode=count
