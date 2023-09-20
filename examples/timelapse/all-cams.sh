#!/bin/bash
pkill -f timelapse
./timelapse -w 3264 -h 2448 -c 0 &
./timelapse -w 3264 -h 2448 -c 1 &
./timelapse -w 3264 -h 2448 -c 2 &
./timelapse -w 3264 -h 2448 -c 3 &
./timelapse -w 3264 -h 2448 -c 4 &
./timelapse -w 3264 -h 2448 -c 5 &
./timelapse -w 3264 -h 2448 -c 6 &
./timelapse -w 3264 -h 2448 -c 7 &
./timelapse -w 3264 -h 2448 -c 8 &
./timelapse -w 3264 -h 2448 -c 9 &
