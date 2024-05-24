#
# build the target application
#
FROM public.ecr.aws/docker/library/golang:1.22.3-alpine3.20 as builder

# update the packages
RUN apk update && apk upgrade && apk add --no-cache make

WORKDIR /build
COPY go.mod go.sum Makefile ./
COPY cmd ./cmd
ARG GIT_COMMIT
RUN make rebuild-docker GIT_COMMIT="$GIT_COMMIT"

#
# build the target container
#
FROM public.ecr.aws/docker/library/alpine:3.20

# update the packages
RUN apk update && apk upgrade && apk add bash tzdata ca-certificates imagemagick msttcorefonts-installer curl && rm -rf /var/cache/apk/* && update-ms-fonts

# Create the run user and group
RUN addgroup webservice && adduser webservice -G webservice -D

# set the timezone appropriatly
ENV TZ=UTC
RUN cp /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Specify home 
ENV APP_HOME /pdf-ws
WORKDIR $APP_HOME

# Create necessary directories
RUN mkdir -p $APP_HOME/scripts $APP_HOME/bin $APP_HOME/web $APP_HOME/assets
RUN chown -R webservice $APP_HOME && chgrp -R webservice $APP_HOME

# port and run command
EXPOSE 8088
CMD scripts/entry.sh

# Move in necessary assets
COPY package/data/container_bash_profile /home/webservice/.profile
COPY package/scripts/entry.sh scripts/* $APP_HOME/scripts/
COPY web/* $APP_HOME/web/
COPY assets/* $APP_HOME/assets/
COPY --from=builder /build/bin/pdf-ws.linux $APP_HOME/bin/pdf-ws

# Ensure permissions are correct
RUN chown webservice:webservice /home/webservice/.profile $APP_HOME/scripts/entry.sh $APP_HOME/bin/pdf-ws && chmod 755 /home/webservice/.profile $APP_HOME/scripts/entry.sh $APP_HOME/bin/pdf-ws

# Add the build tag
ARG BUILD_TAG
RUN test -n "$BUILD_TAG" && touch $APP_HOME/buildtag.build-$BUILD_TAG || touch $APP_HOME/buildtag.build-0

# Specify the user
USER webservice

#
# end of file
#
