// Implementation of CyVideoDecoder that uses WebCodecs API.

import { Codecs, type CyVideoDecoder, type ParsedPacket } from "./videoDecoders";

export async function createWebCodecsDecoder(codec: Codecs): Promise<CyVideoDecoder> {
	if (!("VideoDecoder" in window)) {
		throw new Error("WebCodecs API is not supported in this browser.");
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

	let codecName = "";
	switch (codec) {
		case Codecs.H264:
			codecName = "H264";
			break;
		case Codecs.H265:
			codecName = "hev1.1.6.L93.B0"; // Just a guess from o3
			break;
	}

	await decoder.configure({
		codec: codecName,
		hardwareAcceleration: "prefer-hardware",
	});

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
