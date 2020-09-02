# run application

secs="30"

while true; do
	./bin/pdf-ws

	echo
	echo "*** program exited; restarting in $secs seconds ***"
	echo
	sleep $secs
done

#
# end of file
#
