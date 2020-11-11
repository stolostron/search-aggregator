#!/bin/bash

echo " > Running run-unit-tests.sh"
set -e
export DOCKER_IMAGE_AND_TAG=${1}

make test
make coverage
make lint

exit 0