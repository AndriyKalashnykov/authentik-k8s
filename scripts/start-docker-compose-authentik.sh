#!/bin/bash

LAUNCH_DIR=$(pwd); SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; cd $SCRIPT_DIR; cd ..; SCRIPT_PARENT_DIR=$(pwd);

cd $SCRIPT_PARENT_DIR/compose
echo $PWD

docker-compose down --volumes
rm -rf certs/
rm -rf custom-templates/
rm -rf media/

docker-compose pull
docker-compose up

xdg-open https://localhost:9443/

cd $LAUNCH_DIR
