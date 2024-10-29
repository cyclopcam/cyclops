import { AnalysisState, DetectionResult } from "@/camera/nn";
import { globals } from "@/globals";

export function drawAnalyzedObjects(can: HTMLCanvasElement, cx: CanvasRenderingContext2D, detection: AnalysisState) {
	if (!detection.input)
		return;
	let sx = can.width / detection.input.imageWidth;
	let sy = can.height / detection.input.imageHeight;
	for (let d of detection.objects) {
		if (d.genuine) {
			cx.lineWidth = 4;
			cx.strokeStyle = "#f00";
			cx.font = 'bold 18px sans-serif';
		} else {
			cx.lineWidth = 2;
			cx.strokeStyle = "#fc0";
			cx.font = '18px sans-serif';
		}
		cx.strokeRect(d.box.x * sx, d.box.y * sy, d.box.width * sx, d.box.height * sy);
		cx.fillStyle = '#fff';
		cx.textAlign = 'left';
		cx.textBaseline = 'top';
		cx.fillText(globals.objectClasses[d.class] + ' ' + d.id, d.box.x * sx, d.box.y * sy);
	}
}

export function drawRawNNObjects(can: HTMLCanvasElement, cx: CanvasRenderingContext2D, detection: DetectionResult) {
	let sx = can.width / detection.imageWidth;
	let sy = can.height / detection.imageHeight;
	for (let d of detection.objects) {
		cx.lineWidth = 2;
		cx.strokeStyle = "#0c0";
		cx.font = '18px sans-serif';
		cx.strokeRect(d.box.x * sx, d.box.y * sy, d.box.width * sx, d.box.height * sy);
		cx.fillStyle = '#fff';
		cx.textAlign = 'left';
		cx.textBaseline = 'top';
		cx.fillText(globals.objectClasses[d.class], d.box.x * sx, d.box.y * sy);
	}
}

