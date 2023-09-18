#!/bin/bash
pkill -f webcam
./webcam -p ":9090" -d "/dev/video0" &
./webcam -p ":9091" -d "/dev/video1" &
./webcam -p ":9092" -d "/dev/video2" &
./webcam -p ":9093" -d "/dev/video3" &
./webcam -p ":9094" -d "/dev/video4" &
./webcam -p ":9095" -d "/dev/video5" &
./webcam -p ":9096" -d "/dev/video6" &
./webcam -p ":9097" -d "/dev/video7" &
./webcam -p ":9098" -d "/dev/video8" &
./webcam -p ":9099" -d "/dev/video9" &
