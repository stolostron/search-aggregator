#!/bin/bash

echo " > Running run-unit-tests.sh"
set -e
export DOCKER_IMAGE_AND_TAG=${1}

pwd
ls
ls test-data
make deps
make test
make coverage
make lint

exit 0