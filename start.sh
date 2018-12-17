echo "Starting HaloDEX Chart Feed in background"
#go run *.go
./builds/halodex-chart-feed-ubuntu64 >> debug.log &2> error.log
#tail -f -n 40 debug.log

