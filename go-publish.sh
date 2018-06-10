#!/usr/bin/env bash

# Make sure your jfrog config env is set to repo.jfrog.org by default
# $ jfrog rt c show
# Server ID: rjo
# Url: https://repo.jfrog.org/artifactory/
# API key: AK....
# Default:  true

version=$1
if [ -z "$version" ]; then
    echo "ERROR: Please provide a version. Usage $0 version"
    exit 2
fi

if [[ $version != v* ]]; then
    echo "ERROR: Please provide a version that starts with v . Usage $0 version"
    exit 3
fi

for f in crypto fanout io lru parallel; do
    cd $f && jfrog rt gp go-local $version && cd ..
    RET=$?
    if [ $RET -ne 0 ]; then
        echo "ERROR: Publishing $f failed"
        exit $RET
    fi
done

