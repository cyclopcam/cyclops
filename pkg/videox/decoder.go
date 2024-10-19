package videox

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"
	"unsafe"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/accel"
)

// #cgo pkg-config: libavformat libavcodec libavutil libswscale
// #include <libavformat/avformat.h>
// #include <libavcodec/avcodec.h>
// #include <libavutil/imgutils.h>
// #include <libswscale/swscale.h>
// #include "encoder.h"
import "C"

func frameData(frame *C.AVFrame) **C.uint8_t {
	return (**C.uint8_t)(unsafe.Pointer(&frame.data[0]))
}

func frameLineSize(frame *C.AVFrame) *C.int {
	return (*C.int)(unsafe.Pointer(&frame.linesize[0]))
}

func IsVisualPacket(t h264.NALUType) bool {
	return int(t) >= 1 && int(t) <= 5
}

var ErrResourceTemporarilyUnavailable = errors.New("Resource temporarily unavailable") // common response from avcodec_receive_frame if a frame is not available

func WrapAvErr(err C.int) error {
	if err == -11 {
		return ErrResourceTemporarilyUnavailable
	}
	// We need a better way than this to bring these constants in
	if err == -541478725 {
		return io.EOF
	}
	//char msg[AV_ERROR_MAX_STRING_SIZE] = {0};
	//av_make_error_string(msg, AV_ERROR_MAX_STRING_SIZE, e);
	//return msg;
	//C.av_make_error_string()
	raw := C.GetAvErrorStr(err)
	msg := C.GoString(raw)
	C.free(unsafe.Pointer(raw))
	//fmt.Printf("Error %v: %v\n", err, msg)
	return errors.New(msg)
}

// If you're decoding a file, provide the filename.
// If you're decoding a stream, provide the codec
type DecoderOptions struct {
	Codec    Codec
	Filename string
}

// VideoDecoder is a wrapper around ffmpeg
type VideoDecoder struct {
	formatCtx   *C.AVFormatContext // Only non-nil if we're decoding a file. If decoding an RTSP stream from memory, this is nil.
	ownCodecCtx bool               // True if we created codecCtx, and need to free it.
	videoStream C.int              // Only populated for files
	codecCtx    *C.AVCodecContext
	srcFrame    *C.AVFrame
	swsCtx      *C.struct_SwsContext
	dstFrame    *C.AVFrame
	dstFramePtr []uint8
}

// Create a new decoder that you will feed with packets
func NewVideoStreamDecoder(codec Codec) (*VideoDecoder, error) {
	return NewVideoDecoder(DecoderOptions{
		Codec: codec,
	})
}

// Create a new decoder that will decode the given file
func NewVideoFileDecoder(filename string) (*VideoDecoder, error) {
	return NewVideoDecoder(DecoderOptions{
		Filename: filename,
	})
}

// NewVideoDecoder allocates a new VideoDecoder.
func NewVideoDecoder(options DecoderOptions) (*VideoDecoder, error) {
	// I tried this on Rpi4, to make sure I'm getting hardware decode.. but:
	// This doesn't work.. I just get "avcodec_receive_frame error Resource temporarily unavailable"
	// Perhaps it could work, I didn't try harder.
	//codecName := C.CString("h264_v4l2m2m")
	//defer C.free(unsafe.Pointer(codecName))
	//codec := C.avcodec_find_decoder_by_name(codecName)

	var formatCtx *C.AVFormatContext
	var codecCtx *C.AVCodecContext
	var codec *C.AVCodec
	ownCodecCtx := false
	success := false
	pSuccess := &success
	videoStream := C.int(-1)

	if options.Filename != "" {
		if cerr := C.avformat_open_input(&formatCtx, C.CString(options.Filename), nil, nil); cerr < 0 {
			return nil, fmt.Errorf("Failed to open video file %v: %w", options.Filename, WrapAvErr(cerr))
		}
		defer func() {
			if !*pSuccess {
				C.avformat_close_input(&formatCtx)
			}
		}()

		if cerr := C.avformat_find_stream_info(formatCtx, nil); cerr < 0 {
			return nil, fmt.Errorf("Failed to find stream info for video file %v: %w", options.Filename, WrapAvErr(cerr))
		}
		videoStream = C.av_find_best_stream(formatCtx, C.AVMEDIA_TYPE_VIDEO, -1, -1, &codec, 0)
		if videoStream < 0 {
			return nil, fmt.Errorf("Failed to find video stream or codec in %v: %w", options.Filename, WrapAvErr(videoStream))
		}
		codecPar := unsafe.Slice((**C.AVStream)(formatCtx.streams), formatCtx.nb_streams)[videoStream].codecpar
		//codecCtx = unsafe.Slice((**C.AVStream)(formatCtx.streams), formatCtx.nb_streams)[videoStream].codec
		codecCtx = C.avcodec_alloc_context3(codec)
		if codecCtx == nil {
			return nil, fmt.Errorf("avcodec_alloc_context3() failed")
		}
		defer func() {
			if !*pSuccess {
				C.avcodec_close(codecCtx)
			}
		}()
		ownCodecCtx = true
		if C.avcodec_parameters_to_context(codecCtx, codecPar) < 0 {
			return nil, fmt.Errorf("avcodec_parameters_to_context() failed")
		}
	} else {
		if options.Codec != CodecH264 && options.Codec != CodecH265 {
			return nil, fmt.Errorf("Only h264 and h265 codecs are supported by VideoDecoder")
		}

		// Note: There is also avcodec_find_decoder_by_name()
		avcodec_id := uint32(C.AV_CODEC_ID_H264)
		if options.Codec == CodecH265 {
			avcodec_id = uint32(C.AV_CODEC_ID_H265)
		}
		codec = C.avcodec_find_decoder(avcodec_id)
		if codec == nil {
			return nil, fmt.Errorf("avcodec_find_decoder() failed")
		}

		codecCtx = C.avcodec_alloc_context3(codec)
		if codecCtx == nil {
			return nil, fmt.Errorf("avcodec_alloc_context3() failed")
		}
		defer func() {
			if !*pSuccess {
				C.avcodec_close(codecCtx)
			}
		}()
		ownCodecCtx = true
	}

	if cerr := C.avcodec_open2(codecCtx, codec, nil); cerr < 0 {
		return nil, fmt.Errorf("avcodec_open2() failed: %v", WrapAvErr(cerr))
	}

	srcFrame := C.av_frame_alloc()
	if srcFrame == nil {
		return nil, fmt.Errorf("av_frame_alloc() failed")
	}

	decoder := &VideoDecoder{
		ownCodecCtx: ownCodecCtx,
		codecCtx:    codecCtx,
		videoStream: videoStream,
		formatCtx:   formatCtx,
		srcFrame:    srcFrame,
	}

	success = true

	return decoder, nil
}

// Close closes the decoder.
func (d *VideoDecoder) Close() {
	if d.dstFrame != nil {
		C.av_frame_free(&d.dstFrame)
	}

	if d.swsCtx != nil {
		C.sws_freeContext(d.swsCtx)
	}

	C.av_frame_free(&d.srcFrame)
	if d.ownCodecCtx {
		C.avcodec_close(d.codecCtx)
	}
	if d.formatCtx != nil {
		C.avformat_close_input(&d.formatCtx)
	}
}

// NextFrame reads the next frame from a file and returns a copy of the YUV image.
func (d *VideoDecoder) NextFrame() (*accel.YUVImage, error) {
	img, err := d.NextFrameDeepRef()
	if err != nil {
		return nil, err
	}
	return img.Clone(), nil
}

// NextFrameDeepRef will read the next frame from a file and return a deep
// reference into the libavcodec decoded image buffer.
// The next call to NextFrame/NextFrameDeepRef will invalidate that image.
func (d *VideoDecoder) NextFrameDeepRef() (*accel.YUVImage, error) {
	packet := C.av_packet_alloc()
	defer C.av_packet_free(&packet)

	for {
		if cerr := C.av_read_frame(d.formatCtx, packet); cerr < 0 {
			return nil, WrapAvErr(cerr)
		}

		sendPacketErr := C.int(0)
		if packet.stream_index == d.videoStream {
			sendPacketErr = C.avcodec_send_packet(d.codecCtx, packet)
		}

		C.av_packet_unref(packet)

		if sendPacketErr < 0 {
			return nil, WrapAvErr(sendPacketErr)
		}

		// Note that we're not supporting non-trivial codecs here that might need multiple calls to
		// receive_frame before a frame is available. Also, we're assuming that PTS is monotonically
		// increasing.
		cerr := C.avcodec_receive_frame(d.codecCtx, d.srcFrame)
		if cerr == 0 {
			// Extract time out of d.srcFrame.pts
			//var pts time.Time
			//if d.srcFrame.pts != 0 {
			//	pts = time.UnixMilli(int64(d.srcFrame.pts) * int64(d.formatCtx.streams[d.videoStream].time_base.num) / int64(d.formatCtx.streams[d.videoStream].time_base.den))
			//}
			//dumpYUV420pFrame(d.srcFrame)
			deepRef := makeYUV420ImageDeepUnsafeReference(d.srcFrame)
			return &deepRef, nil
		} else {
			return nil, WrapAvErr(cerr)
		}
	}
}

// Send the packet to the decoder, but don't retrieve the next frame
// This is of dubious value... I wanted to use it for decoding SPS only.. but turns out
// you must decode the first frame before avcodec context will have the width/height.
// So I ended up getting a dedicated SPS parser...
//func (d *H264Decoder) DecodeAndDiscard(nalu NALU) error {
//	return d.sendPacket(nalu)
//}

// Decode the packet and return a copy of the YUV image.
// This is used when decoding a stream (not a file).
func (d *VideoDecoder) Decode(packet *VideoPacket) (*accel.YUVImage, error) {
	img, err := d.DecodeDeepRef(packet)
	if err != nil {
		return nil, err
	}
	return img.Clone(), nil
}

// WARNING: The image returned is only valid while the decoder is still alive,
// and it will be clobbered by the subsequent DecodeDeepRef/Decode().
// The pixels in the returned image are not a garbage-collected Go slice.
// They point directly into the libavcodec decode buffer.
// That's why the function name has the "DeepRef" suffix.
func (d *VideoDecoder) DecodeDeepRef(packet *VideoPacket) (*accel.YUVImage, error) {
	// This was an experiment to see if the h264 codec would accept RBSP packets
	// (i.e. without any start code or emulation prevention bytes).
	// The answer is a resounding NO.
	//for _, p := range packet.H264NALUs {
	//	d.sendPacket(p.Payload)
	//}

	if err := d.sendPacket(packet.EncodeToAnnexBPacket()); err != nil {
		// sendPacket failure is not fatal
		// We should log it or something.
		// But it occurs normally during start of a stream, before first IDR has been seen.
	}

	//if !IsVisualPacket(nalu.Type()) {
	//	// avcodec_receive_frame will return an error if we try to decode a frame when
	//	// sending a non-visual NALU
	//	return nil, nil
	//}

	// receive frame if available
	res := C.avcodec_receive_frame(d.codecCtx, d.srcFrame)
	if res < 0 {
		// This special code path should no longer be necessary. We were decoding before SPS+PPS,
		// and we were trying to extract a frame when sending future SPS+PPS.. so both those paths
		// are now removed, and any error should be a genuine error.
		//err := WrapAvErr(res)
		//if err == ErrResourceTemporarilyUnavailable {
		//	// is this missing SPS + PPS prefixed to IDR?
		//	return nil, nil
		//}
		return nil, fmt.Errorf("avcodec_receive_frame error %w", WrapAvErr(res))
	}

	deepRef := makeYUV420ImageDeepUnsafeReference(d.srcFrame)
	return &deepRef, nil

	// The code below all works, and was originally used when we decoded to RGB.
	// Subsequently, it became clear that it was more useful to get a YUV image out,
	// because then we have a gray channel already crafted for us, which we can use
	// for things like simple motion detection.
	// In addition, we skip the small, but not zero cost, of converting to RGB for
	// frames that nobody will ever see. At least, this is true for the case where
	// your computer is unable to run the neural network on every single frame.
	//
	// At some point, it might be worthwhile seeing of the ffmpeg scaling code
	// does a better job of scaling to our desired NN resolution, than doing it
	// ourselves, after the YUV-to-RGB conversion.

	/*
		useLibSimdForYUVtoRGB := true

		if useLibSimdForYUVtoRGB {
			width := int(d.srcFrame.width)
			height := int(d.srcFrame.height)
			strideY := int(d.srcFrame.linesize[0])
			strideU := int(d.srcFrame.linesize[1])
			strideV := int(d.srcFrame.linesize[2])
			rawY := unsafe.Slice((*byte)(d.srcFrame.data[0]), strideY)
			rawU := unsafe.Slice((*byte)(d.srcFrame.data[1]), strideU)
			rawV := unsafe.Slice((*byte)(d.srcFrame.data[2]), strideV)
			img := cimg.NewImage(width, height, cimg.PixelFormatRGB)
			start := time.Now()
			accel.YUV420pToRGB(width, height, rawY, rawU, rawV, strideY, strideU, strideV, img.Stride, img.Pixels)
			perfstats.UpdateMovingAverage(&perfstats.Stats.YUV420ToRGB_NanosecondsPerKibiPixel, (time.Since(start).Nanoseconds()*1024)/(int64(d.srcFrame.width*d.srcFrame.height)))
			return img, nil
		} else {
			// if frame size has changed, allocate needed objects
			if d.dstFrame == nil || d.dstFrame.width != d.srcFrame.width || d.dstFrame.height != d.srcFrame.height {
				if d.dstFrame != nil {
					C.av_frame_free(&d.dstFrame)
				}

				if d.swsCtx != nil {
					C.sws_freeContext(d.swsCtx)
				}

				d.dstFrame = C.av_frame_alloc()
				d.dstFrame.format = C.AV_PIX_FMT_RGB24
				d.dstFrame.width = d.srcFrame.width
				d.dstFrame.height = d.srcFrame.height
				d.dstFrame.color_range = C.AVCOL_RANGE_JPEG
				res = C.av_frame_get_buffer(d.dstFrame, 1)
				if res < 0 {
					return nil, fmt.Errorf("av_frame_get_buffer() error %v", res)
				}

				d.swsCtx = C.sws_getContext(d.srcFrame.width, d.srcFrame.height, C.AV_PIX_FMT_YUV420P,
					d.dstFrame.width, d.dstFrame.height, (int32)(d.dstFrame.format), C.SWS_BILINEAR, nil, nil, nil)
				if d.swsCtx == nil {
					return nil, fmt.Errorf("sws_getContext() error")
				}

				dstFrameSize := C.av_image_get_buffer_size((int32)(d.dstFrame.format), d.dstFrame.width, d.dstFrame.height, 1)
				d.dstFramePtr = (*[1 << 30]uint8)(unsafe.Pointer(d.dstFrame.data[0]))[:dstFrameSize:dstFrameSize]
			}

			//dumpYUV420pFrame(d.srcFrame)

			// convert frame from YUV420 to RGB
			start := time.Now()
			res = C.sws_scale(d.swsCtx, frameData(d.srcFrame), frameLineSize(d.srcFrame),
				0, d.srcFrame.height, frameData(d.dstFrame), frameLineSize(d.dstFrame))
			if res < 0 {
				return nil, fmt.Errorf("sws_scale() error %v", res)
			}
			perfstats.UpdateMovingAverage(&perfstats.Stats.YUV420ToRGB_NanosecondsPerKibiPixel, (time.Since(start).Nanoseconds()*1024)/(int64(d.srcFrame.width*d.srcFrame.height)))
			//fmt.Printf("Got frame %v x %v -> %v x %v\n", d.srcFrame.width, d.srcFrame.height, d.dstFrame.width, d.dstFrame.height)
			return cimg.WrapImage(int(d.dstFrame.width), int(d.dstFrame.height), cimg.PixelFormatRGB, d.dstFramePtr), nil
		}
	*/
}

func (d *VideoDecoder) Width() int {
	return int(d.codecCtx.width)
}

func (d *VideoDecoder) Height() int {
	return int(d.codecCtx.height)
}

func (d *VideoDecoder) sendPacket(packet []byte) error {
	// The following doesn't work, because we're storing a Go pointer inside a C struct,
	// and then passing that C struct to a C function. So to fix this, we need to define
	// a helper function in C.
	// d.avPacket.data = (*C.uint8_t)(unsafe.Pointer(&nalu.Payload[0]))
	// This is why I created AvCodecSendPacket.
	// Using a Go struct for the C struct is anyway not future compatible, because the avcodec
	// API is deprecating the fixed struct size, and rather moving to requiring the use of
	// an alloc function to create a packet.

	//res := C.AvCodecSendPacket(d.codecCtx, unsafe.Pointer(&nalu.Payload[0]), C.ulong(len(nalu.Payload)))
	res := C.AvCodecSendPacket(d.codecCtx, unsafe.Pointer(&packet[0]), C.ulong(len(packet)))
	if res < 0 {
		return fmt.Errorf("avcodec_send_packet failed: %v", res)
	}
	return nil
}

// Creates a decoder and attempts to decode a single IDR packet.
// This was built for extracting a thumbnail during a long recording.
// Obviously this is a bit expensive, because you're creating a decoder
// for just a single frame.
func DecodeSinglePacketToImage(codec Codec, packet *VideoPacket) (*cimg.Image, error) {
	decoder, err := NewVideoStreamDecoder(codec)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	img, err := decoder.Decode(packet)
	if err != nil {
		return nil, err
	}
	return img.ToCImageRGB(), nil
}

// Decode the list of packets, and return the first image that successfully decodes
func DecodeFirstImageInPacketList(codec Codec, packets []*VideoPacket) (*cimg.Image, time.Time, error) {
	return DecodeClosestImageInPacketList(codec, packets, time.Time{}, nil, "")
}

// If true, report the decode FPS
const DebugVideoDecodeTimes = false

// Decode the list of packets, and return the decoded image who's presentation time is closest to targetTime.
// If targetTime is zero, then we return the first image coming out of the decoder.
// If cache is not nil, then we will insert/query the provided cache.
// videoCacheKey is the key for this video. We use {videoCacheKey-PTS} as the complete cache key.
func DecodeClosestImageInPacketList(codec Codec, packets []*VideoPacket, targetTime time.Time, cache *FrameCache, videoCacheKey string) (*cimg.Image, time.Time, error) {
	// First see if the frame is in the cache
	if cache != nil {
		frameCacheKey := cache.MakeKey(videoCacheKey, targetTime.UnixMilli())
		if img := cache.GetFrame(frameCacheKey); img != nil {
			//fmt.Printf("Cache hit for %v\n", frameCacheKey)
			return img.ToCImageRGB(), targetTime, nil
		}
	}

	startTime := time.Now()
	nFramesDecoded := 0
	decoder, err := NewVideoStreamDecoder(codec)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer decoder.Close()
	var bestImg *accel.YUVImage
	bestTime := time.Time{}
	bestDelta := time.Duration(1<<63 - 1)
	var firstError error
	for _, p := range packets {
		img, err := decoder.DecodeDeepRef(p)
		if err != nil && firstError == nil {
			firstError = err
		}
		if img != nil {
			nFramesDecoded++
			if cache != nil {
				frameCacheKey := cache.MakeKey(videoCacheKey, p.WallPTS.UnixMilli())
				cache.AddFrame(frameCacheKey, img.Clone())
			}
			timeDelta := time.Duration(0)
			if !targetTime.IsZero() {
				timeDelta = p.WallPTS.Sub(targetTime)
				if timeDelta < 0 {
					timeDelta = -timeDelta
				}
			}
			if timeDelta < bestDelta {
				bestTime = p.WallPTS
				bestDelta = timeDelta
				bestImg = img.Clone()
			}
			if p.WallPTS.After(targetTime) || targetTime.IsZero() || bestDelta == 0 {
				// No point decoding packets once we've passed our desired time, or if we're 100% on our desired time
				break
			}
		}
	}
	if DebugVideoDecodeTimes && nFramesDecoded != 0 {
		fmt.Printf("Decoded %v frames in %.3f seconds (%.1f FPS)\n", nFramesDecoded, time.Since(startTime).Seconds(), float64(nFramesDecoded)/time.Since(startTime).Seconds())
	}
	if bestImg != nil {
		return bestImg.ToCImageRGB(), bestTime, nil
	}
	if firstError == nil {
		firstError = fmt.Errorf("No image found")
	}
	return nil, time.Time{}, firstError
}

// Create a deep (and unsafe) reference to the YUV 420 frame from ffmpeg.
func makeYUV420ImageDeepUnsafeReference(frame *C.AVFrame) accel.YUVImage {
	width := int(frame.width)
	height := int(frame.height)
	strideY := int(frame.linesize[0])
	strideU := int(frame.linesize[1])
	strideV := int(frame.linesize[2])
	rawY := unsafe.Slice((*byte)(frame.data[0]), strideY*height)
	rawU := unsafe.Slice((*byte)(frame.data[1]), strideU*height/2)
	rawV := unsafe.Slice((*byte)(frame.data[2]), strideV*height/2)
	return accel.YUVImage{
		Width:  width,
		Height: height,
		Y:      rawY,
		U:      rawU,
		V:      rawV,
	}
}

func makeYUVImageCopy(frame *C.AVFrame) *accel.YUVImage {
	ref := makeYUV420ImageDeepUnsafeReference(frame)
	return ref.Clone()
}

func dumpYUV420pFrame(frame *C.AVFrame) {
	fmt.Printf("%v\n", frame)

	dump, _ := os.Create("testdata/yuv/dump.y")
	dump.Write(unsafe.Slice((*byte)(frame.data[0]), int(frame.linesize[0]*frame.height)))
	dump, _ = os.Create("testdata/yuv/dump.u")
	dump.Write(unsafe.Slice((*byte)(frame.data[1]), int(frame.linesize[1]*frame.height/2)))
	dump, _ = os.Create("testdata/yuv/dump.v")
	dump.Write(unsafe.Slice((*byte)(frame.data[2]), int(frame.linesize[2]*frame.height/2)))
}
