import { Rect } from "@/util/rect";

// SYNC-GET-EVENT-DETAILS-JSON
interface GetEventDetailsJSON {
	events: EventJSON[];
	idToString: { [id: number]: string };
}

// SYNC-VIDEODB-EVENT
interface EventJSON {
	id: number;
	time: number; // unix time in milliseconds
	duration: number; // duration in milliseconds
	camera: number; // camera ID
	resolution: [number, number]; // [width, height] of camera stream on which detection was run
	detections: EventDetectionsJSON;
}

// SYNC-VIDEODB-EVENTDETECTIONS
interface EventDetectionsJSON {
	objects: ObjectJSON[];
}

// SYNC-VIDEODB-OBJECT
interface ObjectJSON {
	id: number; // arbitrary ID that can be used to track object across different event records
	class: number; // class ID
	positions: ObjectPositionJSON[];
	numDetections: number;
}

// SYNC-VIDEODB-OBJECTPOSITION
interface ObjectPositionJSON {
	box: [number, number, number, number];
	time: number;
	confidence: number;
}

class CameraEventObject {
	cls: string; // eg "person", "car"
	positions: CameraEventObjectPosition[];

	constructor(cls: string) {
		this.cls = cls;
		this.positions = [];
	}
}

class CameraEventObjectPosition {
	box: Rect;
	time: Date;
	confidence: number;

	constructor(box: Rect, time: Date, confidence: number) {
		this.box = box;
		this.time = time;
		this.confidence = confidence;
	}
}

// CameraEvent is an event that occurred in a camera feed.
// Basically, this means that something of interest was detected in the camera feed.
export class CameraEvent {
	objects: CameraEventObject[];
	resolution: [number, number]; // [width, height] of camera stream on which detection was run

	constructor() {
		this.objects = [];
		this.resolution = [0, 0];
	}

	static async fetchEvents(cameraID: number, startTime: Date, endTime: Date): Promise<CameraEvent[]> {
		let r = await fetch(`/api/events/details?camera=${cameraID}&startTime=${startTime.getTime()}&endTime=${endTime.getTime()}`);
		let j = await r.json() as GetEventDetailsJSON;
		let outEvents: CameraEvent[] = [];
		for (let ev of j.events) {
			let outEvent = new CameraEvent();
			outEvent.resolution = ev.resolution;
			for (let objects of ev.detections.objects) {
				let outObject = new CameraEventObject(j.idToString[objects.class]);
				outObject.positions = objects.positions.map((pos) => {
					return new CameraEventObjectPosition(
						new Rect(pos.box[0], pos.box[1], pos.box[2], pos.box[3]),
						new Date(ev.time + pos.time),
						pos.confidence
					);
				});
				outEvent.objects.push(outObject);
			}
			outEvents.push(outEvent);
		}
		return outEvents;
	}
}