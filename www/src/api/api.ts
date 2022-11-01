// This is a grab-bag of various API definitions from the server, which don't seem to belong anywhere else

// SYNC-SYSTEM-INFO-JSON
export interface SystemInfoJSON {
	readyError?: string;
	cameras: any[]; // CameraInfo.fromJSON takes these in
}
