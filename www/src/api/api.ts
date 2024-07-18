// This is a grab-bag of various API definitions from the server, which don't seem to belong anywhere else

// SYNC-SYSTEM-INFO-JSON
export interface SystemInfoJSON {
	startupErrors: StartupErrorJSON[];
	cameras: any[]; // CameraInfo.fromJSON takes these in
}

// SYNC-STARTUP-ERROR
export interface StartupErrorJSON {
	// SYNC-STARTUP-ERROR-CODES
	code: "ARCHIVE_PATH";
	message: string;
}
