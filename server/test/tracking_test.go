package test

import "testing"

type EventTrackingParams struct {
	ModelName  string  // eg "yolov8m"
	NNCoverage float64 // eg 75%, if we're able to run NN analysis on 75% of video frames (i.e. because we're resource constrained)
}

type EventTrackingTestCase struct {
	VideoFilename string // eg "tracking/0001-LD.mp4"
	NumPeople     int    // Expected number of people
	NumVehicles   int    // Expected number of vehicles
}

func testEventTrackingCase(t *testing.T, params *EventTrackingParams, tcase *EventTrackingTestCase) {
	// TODO!
}

func TestEventTracking(t *testing.T) {
	paramPurmutations := []*EventTrackingParams{
		{
			ModelName:  "yolov8m",
			NNCoverage: 0.7,
		},
		{
			ModelName:  "yolov8m",
			NNCoverage: 0.5,
		},
		{
			ModelName:  "yolov8s",
			NNCoverage: 1,
		},
		{
			ModelName:  "yolov8s",
			NNCoverage: 0.5,
		},
	}
	cases := []*EventTrackingTestCase{
		{
			VideoFilename: "tracking/0001-LD.mp4",
			NumPeople:     1,
			NumVehicles:   0,
		},
	}
	for _, params := range paramPurmutations {
		for _, tcase := range cases {
			testEventTrackingCase(t, params, tcase)
		}
	}
}
