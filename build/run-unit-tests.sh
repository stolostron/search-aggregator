#!/bin/bash
 # Copyright Contributors to the Open Cluster Management project
 
echo " > Running run-unit-tests.sh"
set -e
export DOCKER_IMAGE_AND_TAG=${1}

make deps
make test
make coverage
make lint

exit 0