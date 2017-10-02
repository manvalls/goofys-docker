#!/bin/sh
set -e
PLUGIN_NAME=haibinfx/goofys
PLUGIN_TAG=2.0

MSYS_NO_PATHCONV=1

machine() {
DOCKERMACHINE=$(which docker-machine)||echo ""
if [ -n "$DOCKERMACHINE" ]; then
   MACHINE=$(docker-machine ls --filter state=running --format "{{.Name}}")||echo ""
   eval $(docker-machine env --shell=bash --no-proxy $MACHINE)
fi
}

all() {
clean; img; rootfs; create;
}

clean() {
	echo "### rm ./plugin"
	rm -rf ./plugin
}
img() {
	echo "### docker build: rootfs image with goofys-docker"
	docker build -t ${PLUGIN_NAME}:rootfs . \
	    --build-arg https_proxy=$HTTP_PROXY --build-arg http_proxy=$HTTP_PROXY \
        --build-arg HTTP_PROXY=$HTTP_PROXY --build-arg HTTPS_PROXY=$HTTP_PROXY  \
        --build-arg NO_PROXY=$NO_PROXY  --build-arg no_proxy=$NO_PROXY
}

rootfs() {
	echo "### create rootfs directory in ./plugin/rootfs"
	mkdir -p ./plugin/rootfs

	gid=$(docker container inspect tmp --format "{{.Id}}" || echo "" )
    if [ -n "$gid" ]; then
      docker container rm tmp
    fi

	docker create --name tmp ${PLUGIN_NAME}:rootfs
	docker export tmp | tar -x -C ./plugin/rootfs
	echo "### copy config.json to ./plugin/"
	cp config.json ./plugin/
	docker rm -vf tmp
}
create() {
	echo "### remove existing plugin ${PLUGIN_NAME}:${PLUGIN_TAG} if exists"
	docker plugin rm -f ${PLUGIN_NAME}:${PLUGIN_TAG} || :
	echo "### create new plugin ${PLUGIN_NAME}:${PLUGIN_TAG} from ./plugin"
	docker plugin create ${PLUGIN_NAME}:${PLUGIN_TAG} ./plugin
}

push() {  clean docker rootfs create enable
	echo "### push plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	docker plugin push ${PLUGIN_NAME}:${PLUGIN_TAG}
}

machine
case "$1" in
    "")
	  all
	  ;;

    noimg)
      yes|docker container prune
      yes|docker volume prune

      docker container rm goofysdocker_test_1||:
      docker volume rm goofysdocker_test||:
      clean; rootfs; create;
      ;;
    cp2m)
      if [ -n "$MACHINE" ]; then
        echo "copy files to docker-machine to build the plugin ..."
        docker-machine ssh $MACHINE rm -rf *
        docker-machine scp Dockerfile $MACHINE:~
        docker-machine scp config.json $MACHINE:~
        docker-machine scp make.sh $MACHINE:~
        docker-machine ssh $MACHINE chmod 700 ./make.sh
        echo "use: docker-machine ssh ${MACHINE}"
        echo "use: ./make.sh noimg"
        # doesn't work.  docker-machine ssh ${MACHINE} sh -C ".\/make.sh noimg"
        docker-machine ssh ${MACHINE}
      fi
      ;;
	log)
      docker-machine ssh ${MACHINE} sudo cat "\/var/log/docker.log"
      ;;
    state)
      docker-machine ssh ${MACHINE} sudo cat "\/var/lib/docker/plugins/goofys-state.json"
      ;;
	test)
      echo "### enable plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
      yes|docker container prune
      yes|docker volume prune

      docker container rm goofysdocker_test_1||:
      docker volume rm goofysdocker_test||:

      enabled=$(docker plugin inspect  -f "{{.Enabled}}" ${PLUGIN_NAME}:${PLUGIN_TAG} ||echo "")
      if [ "$enabled" != "true" ]; then
        set -a
          . ./.env
        set +a
        echo "docker plugin set ${PLUGIN_NAME}:${PLUGIN_TAG} ${plugin_env}"
        docker plugin set ${PLUGIN_NAME}:${PLUGIN_TAG} ${plugin_env} ||:
        docker plugin enable ${PLUGIN_NAME}:${PLUGIN_TAG} ||:
      fi
      docker-compose -f test.yml up
      ;;
	*)
    $1
	;;
esac

