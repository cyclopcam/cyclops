// Implementation of CyVideoDecoder that uses WebCodecs API.

// I created this briefly, and didn't try hard to get it to work.
// Chrome on ubuntu 24.04 didn't support the various H265 variants I tried,
// such as hev1.1.6.L93.B0. I didn't try it on any other platforms, or with h264.
// One could no doubt get this to work, and it would probably be faster than
// round-tripping through the native app. We should do that when the user is
// on a secure connection.

import type { CyVideoDecoder, ParsedPacket } from "./videoDecoders";
import { Codecs } from "@/camera/camera";

export async function createWebCodecsDecoder(codec: Codecs): Promise<CyVideoDecoder> {
	if (!("VideoDecoder" in window)) {
		throw new Error("WebCodecs API is not supported in this browser.");
	}

	let codecName = "";
	switch (codec) {
		case Codecs.H264:
			// I couldn't get any of these to work on Chrome 137.0.7151.55 (Official Build) (64-bit) on Ubuntu 24.04.
			// "avc1.42e01f", // baseline
			// "avc1.4d401f", // main
			// "avc1.640028"  // high			
			codecName = "avc1.4d401f";
			break;
		case Codecs.H265:
			codecName = "hev1.1.6.L93.B0"; // Just a guess from o3
			break;
	}
	let config = {
		codec: codecName,
		hardwareAcceleration: "prefer-hardware",
	} as VideoDecoderConfig;

	let status = await VideoDecoder.isConfigSupported(config);
	if (!status.supported) {
		throw new Error(`Codec ${codecName} is not supported by the WebCodecs API.`);
	}

	//let startTime = performance.now();
	let outputFrameQueue: ImageBitmap[] = [];
	let nPackets = 0;

	let decoder = new VideoDecoder({
		output: async (frame) => {
			if (outputFrameQueue.length >= 10) {
				// Limit the queue size to prevent memory issues
				outputFrameQueue.shift();
			}
			// Convert VideoFrame to ImageBitmap
			//let buffer = new Uint8ClampedArray(frame.allocationSize());
			let bitmap = await createImageBitmap(frame);
			outputFrameQueue.push(bitmap);
			//frame.copyTo(buffer, { format: "RGBA" });
			//let imgData = new ImageData(buffer, frame.codedWidth, frame.codedHeight);
			//let bitmap = createImageBitmap(imgData);
			//outputFrameQueue.push(bitmap);
			frame.close();
		},
		error: (e) => {
			console.error("VideoDecoder error:", e);
		},
	});

	await decoder.configure(config);

	return {
		useNextFrame: true,

		close() {
			decoder.close();
		},

		decode(packet: ParsedPacket) {
			nPackets++;
			//let deltaMS = performance.now() - startTime;
			decoder.decode(new EncodedVideoChunk({
				type: packet.keyframe ? "key" : "delta",
				timestamp: nPackets,
				data: packet.video,
				// TODO: try transfer to avoid copy
			}));
		},

		async nextFrame(): Promise<ImageBitmap | null> {
			if (outputFrameQueue.length > 0) {
				return outputFrameQueue.shift()!;
			}
			return null; // No frame available
		},
	};
}
