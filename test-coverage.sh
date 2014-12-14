#!/bin/bash

# Concatenate coverage data for all go packages under here.

if [[ $1 != "" ]]; then
	token="-repotoken=$1"
fi

echo "mode: set" > concat.out
for subdir in $(find ./* -maxdepth 10 -type d ); 
do
	if ls $subdir/*.go &> /dev/null;
	then
		returnval=`go test -coverprofile=profile.out $subdir`
		echo ${returnval}
		if [[ ${returnval} != *FAIL* ]]
		then
    		if [ -f profile.out ]; then
        		cat profile.out | grep -v "mode: set" >> concat.out 
    		fi
    	else
    		exit 1
    	fi	
    fi
done
if [ -n "$COVERALLS" ]
then
	goveralls ${token} -coverprofile=concat.out $COVERALLS
else
	goveralls ${token} -coverprofile=concat.out .
fi

rm -rf ./profile.out
rm -rf ./concat.out
