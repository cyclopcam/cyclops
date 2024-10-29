export class Rect {
	x = 0;
	y = 0;
	width = 0;
	height = 0;

	parseJSON(j: any) {
		this.x = j.x;
		this.y = j.y;
		this.width = j.width;
		this.height = j.height;
	}
}

export class Detection {
	class = 0;
	confidence = 0;
	box = new Rect();
}

// Neural network detection result on a frame
export class DetectionResult {
	cameraID = 0;
	objects: Detection[] = [];
	imageWidth = 1;
	imageHeight = 1;

	static fromJSON(j: any): DetectionResult {
		let dr = new DetectionResult();
		dr.cameraID = j.cameraID;
		dr.imageWidth = j.imageWidth;
		dr.imageHeight = j.imageHeight;

		for (let jo of j.objects) {
			let d = new Detection();
			d.class = jo.class;
			d.confidence = jo.confidence;
			d.box.parseJSON(jo.box);
			dr.objects.push(d);
		}
		return dr;
	}
}

// An object that was detected by the Object Detector, and is now being tracked by a post-process
// SYNC-TRACKED-OBJECT
export class TrackedObject {
	id = 0;
	class = 0;
	box = new Rect();
	genuine = false;

	parseJSON(j: any) {
		this.id = j.id;
		this.class = j.class;
		this.box.parseJSON(j.box);
		this.genuine = j.genuine;
	}
}

// Analysis that runs as a post-process on top of the raw Object Detection neural network
// SYNC-ANALYSIS-STATE
export class AnalysisState {
	cameraID = 0;
	input: DetectionResult | null = null;
	objects: TrackedObject[] = [];

	static fromJSON(j: any): AnalysisState {
		let as = new AnalysisState();
		as.cameraID = j.cameraID;
		as.input = j.input ? DetectionResult.fromJSON(j.input) : null;
		for (let jo of j.objects) {
			let to = new TrackedObject();
			to.parseJSON(jo);
			as.objects.push(to);
		}
		return as;
	}
}
