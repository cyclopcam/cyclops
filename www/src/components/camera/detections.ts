import { AnalysisState, DetectionResult } from "@/camera/nn";
import { globals } from "@/globals";

export enum BoxDrawMode {
	Regular,
	Thin,
}

export function drawAnalyzedObjects(can: HTMLCanvasElement, cx: CanvasRenderingContext2D, detection: AnalysisState, boxDraw: BoxDrawMode) {
	if (!detection.input)
		return;
	let sx = can.width / detection.input.imageWidth;
	let sy = can.height / detection.input.imageHeight;
	let dpr = window.devicePixelRatio;
	for (let d of detection.objects) {
		let cls = globals.objectClasses[d.class];
		// UPDATE: I've decided to move 'abstract class' processing to later in the pipeline,
		// eg like here. So we no longer receive abstract class events, and we must render
		// concrete classes. If we wanted to eg relabel "car" to "vehicle", we'd do that
		// here.
		//if (globals.abstractClasses[cls]) {
		//	// Skip concrete classes, when there is an abstract class equivalent.
		//	// The abstract class is cleaner, because we merge abstract objects together.
		//	// For example, you might have "car" and "truck" detected on the same pixels,
		//	// but since we merge abstract objects together, these two would become just
		//	// one object. If you were to draw "car" and "truck", then you'd end up seeing
		//	// two objects.
		//	continue;
		//}
		let thickness = boxDraw == BoxDrawMode.Regular ? 2 * dpr : 1;
		if (d.genuine) {
			cx.lineWidth = thickness;
			cx.strokeStyle = "#f00";
			cx.font = 'bold 18px sans-serif';
		} else {
			cx.lineWidth = thickness;
			cx.strokeStyle = "#fc0";
			cx.font = '18px sans-serif';
		}
		cx.strokeRect(d.box.x * sx, d.box.y * sy, d.box.width * sx, d.box.height * sy);
		cx.fillStyle = '#fff';
		cx.textAlign = 'left';
		cx.textBaseline = 'bottom';
		//cx.fillText(cls + ' ' + d.id, d.box.x * sx, d.box.y * sy);
		let confidence = Math.round(d.confidence * 100);
		cx.fillText(confidence + "%", d.box.x * sx, d.box.y * sy);
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

