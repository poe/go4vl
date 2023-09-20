package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

var (
	camera *device.Device
	frames <-chan []byte
	fps    uint32 = 30
	pixfmt v4l2.FourCCType
	height = 2448
	width  = 3264

	streamInfo string
)

type PageData struct {
	StreamInfo  string
	StreamPath  string
	ImgWidth    int
	ImgHeight   int
	ControlPath string
}

// servePage reads templated HTML
func servePage(w http.ResponseWriter, r *http.Request) {
	pd := PageData{
		StreamInfo:  streamInfo,
		StreamPath:  fmt.Sprintf("/stream?%d", time.Now().UnixNano()),
		ImgWidth:    width,
		ImgHeight:   height,
		ControlPath: "/control",
	}

	// Start HTTP response
	w.Header().Add("Content-Type", "text/html")
	t, err := template.ParseFiles("timelapse.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// execute and return the template
	w.WriteHeader(http.StatusOK)
	err = t.Execute(w, pd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getFormatType(fmtStr string) v4l2.FourCCType {
	switch strings.ToLower(fmtStr) {
	case "jpeg":
		return v4l2.PixelFmtJPEG
	case "mpeg":
		return v4l2.PixelFmtMPEG
	case "mjpeg":
		return v4l2.PixelFmtMJPEG
	case "h264", "h.264":
		return v4l2.PixelFmtH264
	case "yuyv":
		return v4l2.PixelFmtYUYV
	case "rgb":
		return v4l2.PixelFmtRGB24
	}
	return v4l2.PixelFmtMPEG
}

// start http service
func serveVideoStream(w http.ResponseWriter, req *http.Request) {
	mimeWriter := multipart.NewWriter(w)
	w.Header().Set("Content-Type", fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", mimeWriter.Boundary()))
	partHeader := make(textproto.MIMEHeader)
	partHeader.Add("Content-Type", "image/jpeg")

	var frame []byte
	for frame = range frames {
		if len(frame) == 0 {
			log.Print("skipping empty frame")
		}
		partWriter, err := mimeWriter.CreatePart(partHeader)
		if err != nil {
			log.Printf("failed to create multi-part writer: %s", err)
			return
		}

		if _, err := partWriter.Write(frame); err != nil {
			log.Printf("failed to write image: %s", err)
		}
	}
}

type controlRequest struct {
	Name  string
	Value string
}

func controlVideo(w http.ResponseWriter, req *http.Request) {
	var ctrl controlRequest
	err := json.NewDecoder(req.Body).Decode(&ctrl)
	if err != nil {
		log.Printf("failed to decode control: %s", err)
		return
	}

	val, err := strconv.Atoi(ctrl.Value)
	if err != nil {
		log.Printf("failed to set brightness: %s", err)
		return
	}

	switch ctrl.Name {
	case "brightness":
		if err := camera.SetControlBrightness(int32(val)); err != nil {
			log.Printf("failed to set brightness: %s", err)
			return
		}
	case "contrast":
		if err := camera.SetControlContrast(int32(val)); err != nil {
			log.Printf("failed to set contrast: %s", err)
			return
		}
	case "saturation":
		if err := camera.SetControlSaturation(int32(val)); err != nil {
			log.Printf("failed to set saturation: %s", err)
			return
		}
	}

	log.Printf("applied control %#v", ctrl)

}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {

	camera_number := "0"
	port := ":909" + camera_number
	devString := "video" + camera_number
	devName := "/dev/" + devString
	frameRate := int(fps)
	buffSize := 4
	defaultDev, err := device.Open(devName)
	skipDefault := false
	if err != nil {
		skipDefault = true
	}

	format := "yuyv"
	if !skipDefault {
		pix, err := defaultDev.GetPixFormat()
		if err == nil {
			width = int(pix.Width)
			height = int(pix.Height)
			switch pix.PixelFormat {
			case v4l2.PixelFmtMJPEG:
				format = "mjpeg"
			case v4l2.PixelFmtH264:
				format = "h264"
			default:
				format = "yuyv"
			}
		}
	}

	if err := defaultDev.Close(); err != nil {
		log.Fatalf("failed to close default device: %s", err)
	}

	flag.StringVar(&camera_number, "c", camera_number, "camera number")
	flag.IntVar(&width, "w", width, "capture width")
	flag.IntVar(&height, "h", height, "capture height")
	flag.StringVar(&format, "f", format, "pixel format")
	flag.StringVar(&port, "p", port, "timelapse service port")
	flag.IntVar(&frameRate, "r", frameRate, "frames per second (fps)")
	flag.IntVar(&buffSize, "b", buffSize, "device buffer size")
	flag.Parse()

	port = ":909" + camera_number
	devString = "video" + camera_number
	devName = "/dev/" + devString

	// open camera and setup camera
	camera, err = device.Open(devName,
		device.WithIOType(v4l2.IOTypeMMAP),
		device.WithPixFormat(v4l2.PixFormat{PixelFormat: getFormatType(format), Width: uint32(width), Height: uint32(height), Field: v4l2.FieldAny}),
		device.WithFPS(uint32(frameRate)),
		device.WithBufferSize(uint32(buffSize)),
	)

	if err != nil {
		log.Fatalf("failed to open device: %s", err)
	}
	defer camera.Close()

	caps := camera.Capability()
	log.Printf("device [%s] opened\n", devName)
	log.Printf("device info: %s", caps.String())

	// set device format
	currFmt, err := camera.GetPixFormat()
	if err != nil {
		log.Fatalf("unable to get format: %s", err)
	}
	log.Printf("Current format: %s", currFmt)
	pixfmt = currFmt.PixelFormat
	streamInfo = fmt.Sprintf("%s - %s [%dx%d] %d fps",
		caps.Card,
		v4l2.PixelFormats[currFmt.PixelFormat],
		currFmt.Width, currFmt.Height, frameRate,
	)

	// start capture
	ctx, cancel := context.WithCancel(context.TODO())
	if err := camera.Start(ctx); err != nil {
		log.Fatalf("stream capture: %s", err)
	}
	defer func() {
		cancel()
		camera.Close()
	}()

	// video stream
	frames = camera.GetOutput()

	log.Printf("device capture started (buffer size set %d)", camera.BufferCount())
	log.Printf("starting server on port %s", port)
	log.Println("use url path /timelapse")

	// setup http service
	http.HandleFunc("/timelapse", servePage)     // returns an html page
	http.HandleFunc("/stream", serveVideoStream) // returns video feed
	http.HandleFunc("/control", controlVideo)    // applies video controls
	go func() {
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatal(err)
		}
	}()
	// process frames from capture channel
	go func() {
		count := 0

		for frame := range camera.GetOutput() {
			fileName := fmt.Sprintf("/timelapse/"+devString+"/capture_%d.jpg", count)
			file, err := os.Create(fileName)
			if err != nil {
				log.Printf("failed to create file %s: %s", fileName, err)
				continue
			}
			if _, err := file.Write(frame); err != nil {
				log.Printf("failed to write file %s: %s", fileName, err)
				continue
			}
			log.Printf("Saved file: %s", fileName)
			if err := file.Close(); err != nil {
				log.Printf("failed to close file %s: %s", fileName, err)
			}
			count++
			time.Sleep(250 * time.Millisecond)

		}

	}()

	for {
		time.Sleep(5000 * time.Millisecond)
	}
}
