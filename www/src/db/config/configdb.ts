import { encodeQuery } from "@/util/util";

export enum Permissions {
	Admin = "a",
	Viewer = "v",
}

// SYNC-RECORD-USER
export class UserRecord {
	id = 0;
	username = "";
	name = "";
	permissions = "";

	static fromJSON(j: any): UserRecord {
		let x = new UserRecord();
		x.id = j.id;
		x.username = j.username;
		x.name = j.name;
		x.permissions = j.permissions;
		return x;
	}

	toJSON(): any {
		return {
			id: this.id,
			username: this.username,
			name: this.name,
			permissions: this.permissions,
		};
	}
}

// SYNC-RECORD-CAMERA
export class CameraRecord {
	id = 0;
	model = ""; // eg HikVision (actually CameraModels enum)
	name = ""; // Friendly name
	host = ""; // Hostname such as 192.168.1.33
	port = 0; // if 0, then default is 554
	username = ""; // RTSP username
	password = ""; // RTSP password
	highResURLSuffix = ""; // eg Streaming/Channels/101 for HikVision. Can leave blank if Model is a known type.
	lowResURLSuffix = ""; // eg Streaming/Channels/102 for HikVision. Can leave blank if Model is a known type.
	createdAt = new Date();
	updatedAt = new Date();

	static fromJSON(j: any): CameraRecord {
		let x = new CameraRecord();
		x.id = j.id;
		x.model = j.model;
		x.name = j.name;
		x.host = j.host;
		x.port = j.port;
		x.username = j.username;
		x.password = j.password;
		x.highResURLSuffix = j.highResURLSuffix;
		x.lowResURLSuffix = j.lowResURLSuffix;
		x.createdAt = new Date(j.createdAt);
		x.updatedAt = new Date(j.updatedAt);
		return x;
	}

	static fromJSONArray(j: any): CameraRecord[] {
		let x = [];
		for (let jj of j) {
			x.push(CameraRecord.fromJSON(jj));
		}
		return x;
	}

	toJSON(): any {
		return {
			id: this.id,
			model: this.model,
			name: this.name,
			host: this.host,
			port: this.port,
			username: this.username,
			password: this.password,
			highResURLSuffix: this.highResURLSuffix,
			lowResURLSuffix: this.lowResURLSuffix,
			createdAt: this.createdAt.getTime(),
			updatedAt: this.updatedAt.getTime(),
		};
	}

	clone(): CameraRecord {
		let c = new CameraRecord();
		c.id = this.id;
		c.model = this.model;
		c.name = this.name;
		c.host = this.host;
		c.port = this.port;
		c.username = this.username;
		c.password = this.password;
		c.highResURLSuffix = this.highResURLSuffix;
		c.lowResURLSuffix = this.lowResURLSuffix;
		c.createdAt = this.createdAt;
		c.updatedAt = this.updatedAt;
		return c;
	}

	posterURL(cacheBreaker?: string): string {
		if (cacheBreaker !== undefined) {
			return "/api/camera/latestImage/" + this.id + "?" + encodeQuery({ cacheBreaker: cacheBreaker });
		} else {
			return "/api/camera/latestImage/" + this.id;
		}
	}
}
