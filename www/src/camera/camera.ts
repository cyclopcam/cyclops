export type Resolution = "ld" | "hd";

// SYNC-INTERNAL-CODEC-NAMES
export enum Codecs {
	H264 = "h264",
	H265 = "h265",
}

// A live running stream
export class StreamInfo {
	name = "";
	codec = Codecs.H264;
	fps = 0; // typically something like 10, 15, 30
	width = 0;
	height = 0;
	keyframeInterval = 0; // Number of frames between keyframes. Typically 10, 20, 30, 40, 50, but can be anything

	static fromJSON(name: string, j: any): StreamInfo {
		let c = new StreamInfo();
		// SYNC-STREAM-INFO-JSON
		c.name = name;
		c.codec = j.codec;
		c.fps = j.fps;
		c.keyframeInterval = j.keyframeInterval;
		c.width = j.width;
		c.height = j.height;
		return c;
	}
}

// CameraInfo is data for a live running camera (which is separate from it's configuration data in CameraRecord)
// See camInfoJSON in Go
export class CameraInfo {
	id = 0; // same id as CameraRecord
	name = "";
	ld!: StreamInfo;
	hd!: StreamInfo;

	static fromJSON(j: any): CameraInfo {
		let c = new CameraInfo();
		c.id = j.id;
		c.name = j.name;
		c.ld = StreamInfo.fromJSON("ld", j.ld);
		c.hd = StreamInfo.fromJSON("hd", j.hd);
		return c;
	}
}
