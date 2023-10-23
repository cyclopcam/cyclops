import { globals } from "@/globals";
import { dateTime, fetchOrThrow } from "@/util/util";

export enum VideoResolution {
	Low = "low",
	Medium = "medium",
	High = "high",
}

// SYNC-ARC-VIDEO-RECORD
export class Video {
	constructor(
		public id: number,
		public cameraName: string,
		public createdBy: number,
		public createdAt: Date
	) { }

	static fromJSON(j: any): Video {
		return new Video(
			j.id,
			j.cameraName,
			j.createdBy,
			new Date(j.createdAt),
		);
	}

	static async fetchAll(): Promise<Video[]> {
		let v = await fetchOrThrow("/api/videos/list");
		let j = await v.json();
		return j.map((x: any) => Video.fromJSON(x));
	}

	static makePublicUrl(stub: string): string {
		return globals.publicVideoBaseUrl + stub;
	}

	thumbnailUrl(): string {
		if (globals.publicVideoBaseUrl !== '')
			return Video.makePublicUrl(`/videos/${this.id}/thumb.jpg`);
		else
			return `/api/video/${this.id}/thumbnail`;
	}

	videoUrl(res: VideoResolution): string {
		if (globals.publicVideoBaseUrl !== '')
			return Video.makePublicUrl(`/videos/${this.id}/${res.toLowerCase()}Res.mp4`);
		else
			return `/api/video/${this.id}/video/${res}`;
	}
}
