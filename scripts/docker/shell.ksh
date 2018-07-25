if [ -z "$DOCKER_HOST" ]; then
   echo "ERROR: no DOCKER_HOST defined"
   exit 1
fi

# set the definitions
INSTANCE=pdf-ws
NAMESPACE=uvadave

docker run -ti -p 8387:8088 $NAMESPACE/$INSTANCE /bin/bash -l
