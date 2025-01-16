package configdb

import (
	"encoding/base64"
	"errors"

	"github.com/cyclopcam/cyclops/pkg/mybits"
)

var ErrDetectionZoneDecode = errors.New("DetectionZone decode error")

// Bitmap of camera detection zone (i.e. which areas of the image are important when the system is armed)
type DetectionZone struct {
	Width  int
	Height int
	Active []byte // Bitmap of Width * Height bits. If bit is 1, then objects in that area are important. If bit is 0, then objects in that area are ignored.
}

func NewDetectionZone(width, height int) *DetectionZone {
	return &DetectionZone{
		Width:  width,
		Height: height,
		Active: make([]byte, (width*height+7)/8),
	}
}

func (d *DetectionZone) EncodeBytes() []byte {
	const HeadingSize = 3
	outBuf := make([]byte, HeadingSize+mybits.MaxEncodedBytes(d.Width*d.Height))
	outBuf[0] = byte(0) // version of this data structure
	outBuf[1] = byte(d.Width)
	outBuf[2] = byte(d.Height)
	n, err := mybits.EncodeOnoff(d.Active, outBuf[HeadingSize:])
	if err != nil {
		panic(err)
	}
	return outBuf[:HeadingSize+n]
}

func (d *DetectionZone) EncodeBase64() string {
	return base64.StdEncoding.EncodeToString(d.EncodeBytes())
}

func DecodeDetectionZoneBase64(dzBase64 string) (*DetectionZone, error) {
	raw, err := base64.StdEncoding.DecodeString(dzBase64)
	if err != nil {
		return nil, err
	}
	return DecodeDetectionZoneBytes(raw)
}

func DecodeDetectionZoneBytes(raw []byte) (*DetectionZone, error) {
	const HeadingSize = 3
	if len(raw) < HeadingSize {
		return nil, ErrDetectionZoneDecode
	}
	version := int(raw[0])
	if version != 0 {
		return nil, ErrDetectionZoneDecode
	}
	width := int(raw[1])
	height := int(raw[2])

	bits := make([]byte, (width*height+7)/8)
	nbits, err := mybits.DecodeOnoff(raw[HeadingSize:], bits)
	if err != nil || nbits != int(width*height) {
		return nil, ErrDetectionZoneDecode
	}

	return &DetectionZone{
		Width:  int(width),
		Height: int(height),
		Active: bits,
	}, nil
}