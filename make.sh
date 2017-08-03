#!/bin/sh
set -e
PLUGIN_NAME=fx/goofys
PLUGIN_TAG=latest

all() {
clean; dockerimg; rootfs; create;
}

clean() {
	echo "### rm ./plugin"
	rm -rf ./plugin
}
dockerimg() {
	echo "### docker build: rootfs image with goofys-docker"
	docker build --squash -q -t ${PLUGIN_NAME}:rootfs . \
	    --build-arg https_proxy=$HTTP_PROXY --build-arg http_proxy=$HTTP_PROXY \
        --build-arg HTTP_PROXY=$HTTP_PROXY --build-arg HTTPS_PROXY=$HTTP_PROXY  \
        --build-arg NO_PROXY=$NO_PROXY  --build-arg no_proxy=$NO_PROXY
}

rootfs() {
	echo "### create rootfs directory in ./plugin/rootfs"
	mkdir -p ./plugin/rootfs
	docker create --name tmp ${PLUGIN_NAME}:rootfs
	docker export tmp | tar -x -C ./plugin/rootfs
	echo "### copy config.json to ./plugin/"
	cp config.json ./plugin/
	docker rm -vf tmp
}
create() {
	echo "### remove existing plugin ${PLUGIN_NAME}:${PLUGIN_TAG} if exists"
	docker plugin rm -f ${PLUGIN_NAME}:${PLUGIN_TAG} || true
	echo "### create new plugin ${PLUGIN_NAME}:${PLUGIN_TAG} from ./plugin"
	docker plugin create ${PLUGIN_NAME}:${PLUGIN_TAG} ./plugin
}
enable() {
	echo "### enable plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	docker plugin enable ${PLUGIN_NAME}:${PLUGIN_TAG}
}
push() {  clean docker rootfs create enable
	echo "### push plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	docker plugin push ${PLUGIN_NAME}:${PLUGIN_TAG}
}

if [ -z "$1" ]; then
  set "all"
fi

$1
