#!/bin/sh
set -e
export MSYS_NO_PATHCONV=1
DOCKERMACHINE=$(which docker-machine)
./make.sh dockerimg
if [ -n "$DOCKERMACHINE" ]; then
   echo "copy files to docker-machine to build the plugin ..."
   MACHINE="default"
   docker-machine ssh $MACHINE "rm -rf *"
   docker-machine scp Dockerfile $MACHINE:~
   docker-machine scp config.json $MACHINE:~
   docker-machine scp make.sh $MACHINE:~
   docker-machine ssh $MACHINE chmod 700 ./make.sh
   echo "use: docker-machine ssh"
   echo "use: ./make.sh"
fi
