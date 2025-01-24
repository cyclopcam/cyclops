// MEH!!!
// I give up on this. It's not worth the pain.

interface RecentScroll {
	key: string; // unique identifier for this page
	top: number;
}

export function saveScrollPosition(key: string, top: number) {
	console.log("Saving scroll of", key, top);
	let recentStr = sessionStorage.getItem("scroll");
	let recent = (recentStr ? JSON.parse(recentStr) : []) as RecentScroll[];
	recent = recent.filter((r) => r.key !== key);
	recent.push({ key, top });
	recent = recent.slice(-5);
	sessionStorage.setItem("scroll", JSON.stringify(recent));
}

export function loadScrollPosition(key: string): number {
	let recentStr = sessionStorage.getItem("scroll");
	let recent = (recentStr ? JSON.parse(recentStr) : []) as RecentScroll[];
	let scroll = recent.find((r) => r.key === key);
	//console.log("Loading scroll of", key, scroll);
	return scroll ? scroll.top : 0;
}

export function saveScrollPositionById(key: string, elementId: string) {
	let element = document.getElementById(elementId);
	if (element) {
		saveScrollPosition(key, element.scrollTop);
	}
}

export function loadScrollPositionById(key: string, elementId: string) {
	let element = document.getElementById(elementId);
	if (element) {
		let pos = loadScrollPosition(key);
		if (pos !== 0) {
			element.scrollTo({ top: pos, behavior: "smooth" });
		}
		//element.scrollTop = loadScrollPosition(key);
	}
}

export class ScrollMemory {
	elementId: string;
	key: string;

	constructor(elementId: string) {
		this.elementId = elementId;
		this.key = window.location.pathname;
		//console.log("ScrollMemory", this.key, this.elementId);
	}

	load() {
		loadScrollPositionById(this.key, this.elementId);
	}

	save() {
		saveScrollPositionById(this.key, this.elementId);
	}
}