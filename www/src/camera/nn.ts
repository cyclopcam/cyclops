export class Rect {
	x = 0;
	y = 0;
	width = 0;
	height = 0;
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
			d.box.x = jo.box.x;
			d.box.y = jo.box.y;
			d.box.width = jo.box.width;
			d.box.height = jo.box.height;
			dr.objects.push(d);
		}
		return dr;
	}
}

// COCO classes
export const COCOClasses: string[] = [
	"person",
	"bicycle",
	"car",
	"motorcycle",
	"airplane",
	"bus",
	"train",
	"truck",
	"boat",
	"traffic light",
	"fire hydrant",
	"stop sign",
	"parking meter",
	"bench",
	"bird",
	"cat",
	"dog",
	"horse",
	"sheep",
	"cow",
	"elephant",
	"bear",
	"zebra",
	"giraffe",
	"backpack",
	"umbrella",
	"handbag",
	"tie",
	"suitcase",
	"frisbee",
	"skis",
	"snowboard",
	"sports ball",
	"kite",
	"baseball bat",
	"baseball glove",
	"skateboard",
	"surfboard",
	"tennis racket",
	"bottle",
	"wine glass",
	"cup",
	"fork",
	"knife",
	"spoon",
	"bowl",
	"banana",
	"apple",
	"sandwich",
	"orange",
	"broccoli",
	"carrot",
	"hot dog",
	"pizza",
	"donut",
	"cake",
	"chair",
	"couch",
	"potted plant",
	"bed",
	"dining table",
	"toilet",
	"tv",
	"laptop",
	"mouse",
	"remote",
	"keyboard",
	"cell phone",
	"microwave",
	"oven",
	"toaster",
	"sink",
	"refrigerator",
	"book",
	"clock",
	"vase",
	"scissors",
	"teddy bear",
	"hair drier",
	"toothbrush",
];
