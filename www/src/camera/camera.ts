// A live running stream
export class StreamInfo {
	name = "";
	fps = 0;
	width = 0;
	height = 0;

	static fromJSON(name: string, j: any): StreamInfo {
		let c = new StreamInfo();
		c.name = name;
		c.fps = j.fps;
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
