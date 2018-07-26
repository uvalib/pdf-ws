FROM alpine:3.7

# update the packages
RUN apk update && apk upgrade && apk add bash tzdata ca-certificates imagemagick && rm -fr /var/cache/apk/*

# Create the run user and group
RUN addgroup webservice && adduser webservice -G webservice -D

# set the timezone appropriatly
ENV TZ=UTC
RUN cp /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Specify home 
ENV APP_HOME /pdf-ws
WORKDIR $APP_HOME

# Create necessary directories
RUN mkdir -p $APP_HOME/scripts $APP_HOME/bin

# Specify the user
USER webservice

# port and run command
EXPOSE 8088
CMD scripts/entry.sh

# Move in necessary assets
COPY data/container_bash_profile /home/webservice/.profile
COPY scripts/entry.sh $APP_HOME/scripts/entry.sh
COPY web $APP_HOME/bin/web
COPY bin/pdf-ws.linux $APP_HOME/bin/pdf-ws

# Set ownership
RUN chown -R webservice:webservice $APP_HOME

# Add the build tag
COPY buildtag.* $APP_HOME/
